package command

import (
	"context"
	"errors"
	"strings"

	adminapp "github.com/csw/console/services/admin-api/internal/app/admin"
	domainsync "github.com/csw/console/services/admin-api/internal/domain/sync"
	"github.com/csw/console/services/admin-api/internal/domain/common"
)

type ExecuteSectionSyncCommand struct {
	GameID           string
	SelectedSections []string
	BaselineToken    string
	IncludeDeletes   bool
	OperatorNote     string
}

func NormalizeExecuteSectionSync(cmd ExecuteSectionSyncCommand) (ExecuteSectionSyncCommand, error) {
	sections, err := domainsync.ParseSections(cmd.SelectedSections, true)
	if err != nil {
		return ExecuteSectionSyncCommand{}, unknownSectionErr(err.Error())
	}
	cmd.GameID = strings.TrimSpace(cmd.GameID)
	cmd.BaselineToken = strings.TrimSpace(cmd.BaselineToken)
	cmd.OperatorNote = strings.TrimSpace(cmd.OperatorNote)
	cmd.SelectedSections = sectionsToStrings(sections)
	return cmd, nil
}

func (s *sectionSyncService) Execute(ctx context.Context, cmd ExecuteSectionSyncCommand) (domainsync.ExecuteResult, error) {
	if cmd.GameID == "" {
		return domainsync.ExecuteResult{}, validationErr("gameId 必填")
	}
	if cmd.BaselineToken == "" {
		return domainsync.ExecuteResult{}, validationErr("baselineToken 必填")
	}
	if len(cmd.OperatorNote) > 255 {
		return domainsync.ExecuteResult{}, validationErr("operatorNote 长度不能超过 255")
	}
	sections, err := domainsync.ParseSections(cmd.SelectedSections, true)
	if err != nil {
		return domainsync.ExecuteResult{}, unknownSectionErr(err.Error())
	}
	payload, err := domainsync.ParseAndVerifyBaselineToken(cmd.BaselineToken, s.tokenKey, s.now().UTC())
	if err != nil {
		return domainsync.ExecuteResult{}, validationErr("baselineToken 无效")
	}
	if payload.GameID != cmd.GameID || payload.SourceEnv != domainsync.DefaultSourceEnv() || payload.TargetEnv != domainsync.DefaultTargetEnv() {
		return domainsync.ExecuteResult{}, validationErr("baselineToken 与当前上下文不匹配")
	}
	consumed, consumedJobID, consumedAt, err := s.tx.Repository().IsNonceConsumed(ctx, payload.Nonce)
	if err != nil {
		return domainsync.ExecuteResult{}, err
	}
	if consumed {
		return domainsync.ExecuteResult{}, tokenConsumedErr(consumedJobID, consumedAt)
	}
	ok, err := s.tx.Repository().ResolveGameExists(ctx, domainsync.DefaultTargetEnv(), cmd.GameID)
	if err != nil {
		return domainsync.ExecuteResult{}, err
	}
	if !ok {
		return domainsync.ExecuteResult{}, notFoundErr("目标环境游戏不存在")
	}
	targetSets := map[domainsync.Section][]domainsync.EntityRecord{}
	for _, section := range domainsync.AllSections() {
		_, prodRows, _, loadErr := s.tx.Repository().LoadSectionEntities(ctx, domainsync.DefaultSourceEnv(), domainsync.DefaultTargetEnv(), cmd.GameID, section)
		if loadErr != nil {
			return domainsync.ExecuteResult{}, loadErr
		}
		targetSets[section] = prodRows
	}
	targetHashNow, err := domainsync.HashEntitySets(targetSets)
	if err != nil {
		return domainsync.ExecuteResult{}, err
	}
	if targetHashNow != payload.TargetHashBefore {
		return domainsync.ExecuteResult{}, baselineMismatchErr(payload.TargetHashBefore, targetHashNow)
	}
	if missing := domainsync.ValidateSectionDependencies(sections, targetSets); len(missing) > 0 {
		details := make([]any, 0, len(missing))
		for _, item := range missing {
			details = append(details, map[string]any{
				"section":           string(item.Section),
				"missingDependency": string(item.MissingDependency),
				"entityKey":         item.EntityKey,
			})
		}
		return domainsync.ExecuteResult{}, validationErr("依赖校验失败", details...)
	}

	operatorID := actorFromCtx(ctx)
	if operatorID == 0 {
		if ac, ok := adminapp.AuthContextFrom(ctx); ok {
			operatorID = ac.UserID
		}
	}
	executedAt := s.now().UTC()
	appliedSummary := map[domainsync.Section]domainsync.DiffSummary{}
	skippedDeletes := []domainsync.ExecuteSkippedDelete{}

	txErr := s.tx.InTx(ctx, func(repo SectionSyncRepository) error {
		ordered := domainsync.SortSectionsByTopo(sections)
		for _, section := range ordered {
			if err := repo.ApplySection(ctx, section, cmd.GameID, cmd.IncludeDeletes); err != nil {
				return err
			}
		}
		if err := repo.ConsumeNonce(ctx, payload.Nonce, payload.SyncJobID); err != nil {
			if errors.Is(err, adminapp.ErrConflict) {
				return tokenConsumedErr(payload.SyncJobID, nil)
			}
			return err
		}
		applied, skipped, err := repo.MarkItemsApplied(ctx, payload.SyncJobID, sections, cmd.IncludeDeletes)
		if err != nil {
			return err
		}
		appliedSummary = applied
		skippedDeletes = skipped
		targetAfterSets := map[domainsync.Section][]domainsync.EntityRecord{}
		for _, section := range domainsync.AllSections() {
			_, prodRows, _, loadErr := repo.LoadSectionEntities(ctx, domainsync.DefaultSourceEnv(), domainsync.DefaultTargetEnv(), cmd.GameID, section)
			if loadErr != nil {
				return loadErr
			}
			targetAfterSets[section] = prodRows
		}
		targetHashAfter, err := domainsync.HashEntitySets(targetAfterSets)
		if err != nil {
			return err
		}
		if err := repo.UpdateJobResult(ctx, payload.SyncJobID, domainsync.JobStatusSucceeded, targetHashAfter, cmd.OperatorNote, &executedAt); err != nil {
			return err
		}
		s.writeAudit(ctx, adminapp.AuditEntry{
			ActorID:      operatorID,
			Action:       "sync.execute",
			ResourceType: syncAuditResourceTypeGame,
			ResourceID:   cmd.GameID,
			Env:          common.EnvProduction,
			Detail: map[string]any{
				"selectedSections": sections,
				"includeDeletes":   cmd.IncludeDeletes,
				"sourceHash":       payload.SourceHash,
				"targetHashBefore": payload.TargetHashBefore,
				"targetHashAfter":  targetHashAfter,
				"appliedSummary":   appliedSummary,
			},
		})
		return nil
	})
	if txErr != nil {
		_ = s.tx.Repository().UpdateJobResult(ctx, payload.SyncJobID, domainsync.JobStatusFailed, "", "", &executedAt)
		return domainsync.ExecuteResult{}, mapSectionSyncError(txErr)
	}

	targetAfterSets := map[domainsync.Section][]domainsync.EntityRecord{}
	for _, section := range domainsync.AllSections() {
		_, prodRows, _, loadErr := s.tx.Repository().LoadSectionEntities(ctx, domainsync.DefaultSourceEnv(), domainsync.DefaultTargetEnv(), cmd.GameID, section)
		if loadErr != nil {
			return domainsync.ExecuteResult{}, loadErr
		}
		targetAfterSets[section] = prodRows
	}
	targetHashAfter, err := domainsync.HashEntitySets(targetAfterSets)
	if err != nil {
		return domainsync.ExecuteResult{}, err
	}
	unselected := make([]domainsync.Section, 0)
	selectedSet := map[domainsync.Section]struct{}{}
	for _, sec := range sections {
		selectedSet[sec] = struct{}{}
	}
	for _, sec := range domainsync.AllSections() {
		if _, ok := selectedSet[sec]; !ok {
			unselected = append(unselected, sec)
		}
	}
	return domainsync.ExecuteResult{
		SyncJobID:        payload.SyncJobID,
		GameID:           cmd.GameID,
		SourceEnv:        domainsync.DefaultSourceEnv(),
		TargetEnv:        domainsync.DefaultTargetEnv(),
		Status:           domainsync.JobStatusSucceeded,
		SelectedSections: sections,
		IncludeDeletes:   cmd.IncludeDeletes,
		SourceHash:       payload.SourceHash,
		TargetHashBefore: payload.TargetHashBefore,
		TargetHashAfter:  targetHashAfter,
		AppliedSummary:   appliedSummary,
		Skipped: domainsync.ExecuteSkipped{
			Deletes:           skippedDeletes,
			UnselectedSection: unselected,
		},
		ExecutedAt: executedAt,
	}, nil
}

func (s *sectionSyncService) ListJobs(ctx context.Context, query ListSectionSyncJobsQuery) (domainsync.JobList, error) {
	query.GameID = strings.TrimSpace(query.GameID)
	if query.GameID == "" {
		return domainsync.JobList{}, validationErr("gameId 必填")
	}
	if query.Page <= 0 {
		query.Page = 1
	}
	if query.PageSize <= 0 {
		query.PageSize = 20
	}
	if query.PageSize > 100 {
		query.PageSize = 100
	}
	items, total, err := s.tx.Repository().ListJobsByGame(ctx, query.GameID, query.Page, query.PageSize, strings.TrimSpace(query.Status))
	if err != nil {
		return domainsync.JobList{}, mapSectionSyncError(err)
	}
	return domainsync.JobList{
		Items:    items,
		Page:     query.Page,
		PageSize: query.PageSize,
		Total:    total,
	}, nil
}

