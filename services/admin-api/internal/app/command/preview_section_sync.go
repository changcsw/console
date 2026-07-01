package command

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	adminapp "github.com/csw/console/services/admin-api/internal/app/admin"
	domainsync "github.com/csw/console/services/admin-api/internal/domain/sync"
)

type PreviewSectionSyncCommand struct {
	GameID           string
	SelectedSections []string
	IncludeDeletes   bool
}

type ListSectionSyncJobsQuery struct {
	GameID   string
	Page     int
	PageSize int
	Status   string
}

type SectionSyncError struct {
	Status  int
	Code    string
	Message string
	Details []any
}

func (e *SectionSyncError) Error() string { return e.Message }

const (
	CodeUnknownSection        = "UNKNOWN_SECTION"
	CodeValidation            = "VALIDATION_FAILED"
	CodeBaselineMismatch      = "SYNC_BASELINE_MISMATCH"
	CodeTokenConsumed         = "SYNC_TOKEN_CONSUMED"
	CodeNotFound              = "NOT_FOUND"
	CodeForbidden             = "FORBIDDEN"
	CodeInternal              = "INTERNAL"
	defaultBaselineTokenTTL   = 30 * time.Minute
	defaultBaselineTokenKey   = "sync-default-baseline-secret"
	syncAuditResourceTypeGame = "game"
)

type SectionSyncTxManager interface {
	Repository() SectionSyncRepository
	InTx(ctx context.Context, fn func(repo SectionSyncRepository) error) error
}

type SectionSyncRepository interface {
	ResolveGameExists(ctx context.Context, schema, gameID string) (bool, error)
	LoadSectionEntities(ctx context.Context, sourceSchema, targetSchema, gameID string, section domainsync.Section) ([]domainsync.EntityRecord, []domainsync.EntityRecord, map[string]struct{}, error)
	CreateJob(ctx context.Context, in CreateSyncJobInput) (int64, error)
	AddItems(ctx context.Context, jobID int64, items []SyncJobItemInput) error
	ListJobsByGame(ctx context.Context, gameID string, page, pageSize int, status string) ([]domainsync.JobItem, int, error)
	UpdateJobResult(ctx context.Context, jobID int64, status domainsync.SyncJobStatus, targetHashAfter, operatorNote string, executedAt *time.Time) error
	MarkItemsApplied(ctx context.Context, jobID int64, selected []domainsync.Section, includeDeletes bool) (map[domainsync.Section]domainsync.DiffSummary, []domainsync.ExecuteSkippedDelete, error)
	IsNonceConsumed(ctx context.Context, nonce string) (bool, int64, *time.Time, error)
	ConsumeNonce(ctx context.Context, nonce string, syncJobID int64) error
	ApplySection(ctx context.Context, section domainsync.Section, gameID string, includeDeletes bool) error
}

type CreateSyncJobInput struct {
	GameID           string
	SourceEnv        string
	TargetEnv        string
	SourceHash       string
	TargetHashBefore string
	IncludeDeletes   bool
	OperatorID       int64
	OperatorNote     string
	Status           domainsync.SyncJobStatus
}

type SyncJobItemInput struct {
	Section         domainsync.Section
	EntityType      string
	EntityKey       string
	Op              domainsync.SyncOp
	FieldName       string
	SandboxValue    any
	ProductionValue any
	Masked          bool
	Applied         bool
}

type SectionSyncService interface {
	Preview(ctx context.Context, cmd PreviewSectionSyncCommand) (domainsync.Preview, error)
	Execute(ctx context.Context, cmd ExecuteSectionSyncCommand) (domainsync.ExecuteResult, error)
	ListJobs(ctx context.Context, query ListSectionSyncJobsQuery) (domainsync.JobList, error)
}

type sectionSyncService struct {
	tx       SectionSyncTxManager
	audit    adminapp.AuditSink
	now      func() time.Time
	ttl      time.Duration
	tokenKey []byte
}

func NewSectionSyncService(tx SectionSyncTxManager, audit adminapp.AuditSink, now func() time.Time, tokenKey string) SectionSyncService {
	if now == nil {
		now = time.Now
	}
	if strings.TrimSpace(tokenKey) == "" {
		tokenKey = defaultBaselineTokenKey
	}
	return &sectionSyncService{
		tx:       tx,
		audit:    audit,
		now:      now,
		ttl:      defaultBaselineTokenTTL,
		tokenKey: []byte(tokenKey),
	}
}

func NormalizePreviewSectionSync(cmd PreviewSectionSyncCommand) (PreviewSectionSyncCommand, error) {
	sections, err := domainsync.ParseSections(cmd.SelectedSections, false)
	if err != nil {
		return PreviewSectionSyncCommand{}, unknownSectionErr(err.Error())
	}
	cmd.GameID = strings.TrimSpace(cmd.GameID)
	cmd.SelectedSections = sectionsToStrings(sections)
	return cmd, nil
}

func (s *sectionSyncService) Preview(ctx context.Context, cmd PreviewSectionSyncCommand) (domainsync.Preview, error) {
	if cmd.GameID == "" {
		return domainsync.Preview{}, validationErr("gameId 必填")
	}
	sections, err := domainsync.ParseSections(cmd.SelectedSections, false)
	if err != nil {
		return domainsync.Preview{}, unknownSectionErr(err.Error())
	}
	ok, err := s.tx.Repository().ResolveGameExists(ctx, domainsync.DefaultSourceEnv(), cmd.GameID)
	if err != nil {
		return domainsync.Preview{}, err
	}
	if !ok {
		return domainsync.Preview{}, notFoundErr("游戏不存在")
	}
	sourceSets := make(map[domainsync.Section][]domainsync.EntityRecord, len(sections))
	targetSets := make(map[domainsync.Section][]domainsync.EntityRecord, len(sections))
	diffSections := make([]domainsync.DiffSection, 0, len(sections))
	jobItems := make([]SyncJobItemInput, 0, 64)
	for _, section := range sections {
		sandboxRows, prodRows, maskedFields, loadErr := s.tx.Repository().LoadSectionEntities(
			ctx,
			domainsync.DefaultSourceEnv(),
			domainsync.DefaultTargetEnv(),
			cmd.GameID,
			section,
		)
		if loadErr != nil {
			return domainsync.Preview{}, loadErr
		}
		sourceSets[section] = sandboxRows
		targetSets[section] = prodRows
		diff := domainsync.DiffEntities(section, sandboxRows, prodRows, maskedFields)
		diffSections = append(diffSections, diff)
		for _, change := range diff.Changes {
			jobItems = append(jobItems, SyncJobItemInput{
				Section:         section,
				EntityType:      change.EntityType,
				EntityKey:       change.EntityKey,
				Op:              change.Op,
				FieldName:       change.FieldName,
				SandboxValue:    wrapValue(change.SandboxValue),
				ProductionValue: wrapValue(change.ProductionValue),
				Masked:          change.Masked,
				Applied:         false,
			})
		}
	}
	sourceHash, err := domainsync.HashEntitySets(sourceSets)
	if err != nil {
		return domainsync.Preview{}, err
	}
	targetHashBefore, err := domainsync.HashEntitySets(targetSets)
	if err != nil {
		return domainsync.Preview{}, err
	}
	operatorID := actorFromCtx(ctx)
	previewedAt := s.now().UTC()
	jobID, err := s.tx.Repository().CreateJob(ctx, CreateSyncJobInput{
		GameID:           cmd.GameID,
		SourceEnv:        domainsync.DefaultSourceEnv(),
		TargetEnv:        domainsync.DefaultTargetEnv(),
		SourceHash:       sourceHash,
		TargetHashBefore: targetHashBefore,
		IncludeDeletes:   cmd.IncludeDeletes,
		OperatorID:       operatorID,
		OperatorNote:     "",
		Status:           domainsync.JobStatusPreviewed,
	})
	if err != nil {
		return domainsync.Preview{}, err
	}
	if err := s.tx.Repository().AddItems(ctx, jobID, jobItems); err != nil {
		return domainsync.Preview{}, err
	}
	nonce, err := domainsync.GenerateNonce()
	if err != nil {
		return domainsync.Preview{}, err
	}
	expiresAt := previewedAt.Add(s.ttl)
	token, err := domainsync.BuildBaselineToken(domainsync.BaselineTokenPayload{
		GameID:           cmd.GameID,
		SyncJobID:        jobID,
		SourceEnv:        domainsync.DefaultSourceEnv(),
		TargetEnv:        domainsync.DefaultTargetEnv(),
		SourceHash:       sourceHash,
		TargetHashBefore: targetHashBefore,
		PreviewedAt:      previewedAt,
		ExpiresAt:        expiresAt,
		Nonce:            nonce,
	}, s.tokenKey)
	if err != nil {
		return domainsync.Preview{}, err
	}
	hasDiff := false
	for _, item := range diffSections {
		if len(item.Changes) > 0 {
			hasDiff = true
			break
		}
	}
	return domainsync.Preview{
		GameID:           cmd.GameID,
		SourceEnv:        domainsync.DefaultSourceEnv(),
		TargetEnv:        domainsync.DefaultTargetEnv(),
		SourceHash:       sourceHash,
		TargetHashBefore: targetHashBefore,
		HasDiff:          hasDiff,
		BaselineToken:    token,
		PreviewedAt:      previewedAt,
		ExpiresAt:        expiresAt,
		Sections:         diffSections,
	}, nil
}

func sectionsToStrings(sections []domainsync.Section) []string {
	result := make([]string, 0, len(sections))
	for _, section := range sections {
		result = append(result, string(section))
	}

	return result
}

func wrapValue(v any) map[string]any {
	return map[string]any{"value": v}
}

func actorFromCtx(ctx context.Context) int64 {
	if ac, ok := adminapp.AuthContextFrom(ctx); ok {
		return ac.UserID
	}
	return 0
}

func validationErr(message string, details ...any) *SectionSyncError {
	if details == nil {
		details = []any{}
	}
	return &SectionSyncError{Status: http.StatusBadRequest, Code: CodeValidation, Message: message, Details: details}
}

func unknownSectionErr(message string) *SectionSyncError {
	return &SectionSyncError{Status: http.StatusBadRequest, Code: CodeUnknownSection, Message: message, Details: []any{}}
}

func baselineMismatchErr(expected, actual string) *SectionSyncError {
	return &SectionSyncError{
		Status:  http.StatusConflict,
		Code:    CodeBaselineMismatch,
		Message: "目标环境基线已变化，请先重新预览",
		Details: []any{map[string]any{"field": "targetHashBefore", "expected": expected, "actual": actual}},
	}
}

func tokenConsumedErr(syncJobID int64, consumedAt *time.Time) *SectionSyncError {
	detail := map[string]any{
		"field":           "baselineToken",
		"consumedSyncJobId": syncJobID,
	}
	if consumedAt != nil {
		detail["consumedAt"] = consumedAt.UTC()
	}
	return &SectionSyncError{
		Status:  http.StatusConflict,
		Code:    CodeTokenConsumed,
		Message: "baselineToken 已被消费",
		Details: []any{detail},
	}
}

func notFoundErr(message string) *SectionSyncError {
	return &SectionSyncError{Status: http.StatusNotFound, Code: CodeNotFound, Message: message, Details: []any{}}
}

func mapSectionSyncError(err error) error {
	var appErr *SectionSyncError
	if errors.As(err, &appErr) {
		return appErr
	}
	if errors.Is(err, adminapp.ErrNotFound) {
		return notFoundErr("资源不存在")
	}
	return err
}

func (s *sectionSyncService) writeAudit(ctx context.Context, entry adminapp.AuditEntry) {
	if s.audit == nil {
		return
	}
	if ac, ok := adminapp.AuthContextFrom(ctx); ok {
		entry.ActorID = ac.UserID
	}
	_ = s.audit.Write(ctx, entry)
}
