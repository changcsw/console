package cashier

import (
	"context"
	"net/http"
	"time"

	adminapp "github.com/csw/console/services/admin-api/internal/app/admin"
	domaincashier "github.com/csw/console/services/admin-api/internal/domain/cashier"
	"github.com/csw/console/services/admin-api/internal/domain/common"
)

type Error struct {
	Status  int
	Code    string
	Message string
	Details []any
}

func (e *Error) Error() string { return e.Message }

const (
	CodeValidation          = "VALIDATION_FAILED"
	CodeConflict            = "CONFLICT"
	CodeNotFound            = "NOT_FOUND"
	CodeVersionStateInvalid = "VERSION_STATE_INVALID"
	CodeCurrencyUnsupported = "CURRENCY_NOT_SUPPORTED"
)

func validationErr(msg string, details ...any) *Error {
	if details == nil {
		details = []any{}
	}
	return &Error{Status: http.StatusBadRequest, Code: CodeValidation, Message: msg, Details: details}
}

func conflictErr(msg string) *Error {
	return &Error{Status: http.StatusConflict, Code: CodeConflict, Message: msg, Details: []any{}}
}

func notFoundErr(msg string) *Error {
	return &Error{Status: http.StatusNotFound, Code: CodeNotFound, Message: msg, Details: []any{}}
}

func versionStateErr(msg string) *Error {
	return &Error{Status: http.StatusConflict, Code: CodeVersionStateInvalid, Message: msg, Details: []any{}}
}

func currencyErr(msg string) *Error {
	return &Error{Status: http.StatusBadRequest, Code: CodeCurrencyUnsupported, Message: msg, Details: []any{}}
}

func fieldDetail(field, reason string) any {
	return map[string]string{"field": field, "reason": reason}
}

type UpsertPriceRowInput struct {
	CountryCode  string
	RegionCode   string
	Currency     string
	PriceID      string
	PreTaxAmount string
	TaxRate      string
	EffectiveAt  time.Time
}

type CreateTemplateInput struct {
	TemplateID     string
	TemplateName   string
	FXSyncEnabled  bool
	FXSyncMode     string
	FXSyncSchedule string
}

type CreateVersionInput struct {
	TemplateID    string
	SourceType    domaincashier.SourceType
	SourceVersion int
}

type BindGameCashierProfileInput struct {
	GameID          string
	TemplateID      string
	TemplateVersion int
}

type GameCashierProfileView struct {
	TemplateID             string
	AppliedTemplateVersion int
	SnapshotChecksum       string
	AppliedAt              time.Time
}

type SaveGameCashierPriceOverridesInput struct {
	GameID string
	Items  []domaincashier.GameCashierPriceOverride
}

// FXSyncRunView 汇率同步运行的对外视图：在领域 run 基础上补充候选版本号（对外版本号，
// 非 versions 表内部主键），供 transport 层组装 camelCase DTO。
type FXSyncRunView struct {
	Run              domaincashier.FXSyncRun
	CandidateVersion int
}

type CashierTemplateRepository interface {
	ListTemplates(ctx context.Context, page, pageSize int) ([]domaincashier.PriceTemplate, int, error)
	CreateTemplate(ctx context.Context, item domaincashier.PriceTemplate) (domaincashier.PriceTemplate, error)
	GetTemplateByTemplateID(ctx context.Context, templateID string) (domaincashier.PriceTemplate, error)
	GetTemplateByID(ctx context.Context, templateRowID int64) (domaincashier.PriceTemplate, error)
	ListVersions(ctx context.Context, templateIDRef int64) ([]domaincashier.TemplateVersionRecord, error)
	GetVersionByTemplateAndVersion(ctx context.Context, templateIDRef int64, version int) (domaincashier.TemplateVersionRecord, error)
	GetVersionByID(ctx context.Context, id int64) (domaincashier.TemplateVersionRecord, error)
	GetPublishedVersion(ctx context.Context, templateIDRef int64) (*domaincashier.TemplateVersionRecord, error)
	NextVersion(ctx context.Context, templateIDRef int64) (int, error)
	CreateVersion(ctx context.Context, version domaincashier.TemplateVersionRecord) (domaincashier.TemplateVersionRecord, error)
	ArchiveVersion(ctx context.Context, versionID int64, at time.Time) error
	PublishVersion(ctx context.Context, versionID int64, at time.Time, checksum string) error

	ListRows(ctx context.Context, versionID int64) ([]domaincashier.PriceRow, error)
	ReplaceRows(ctx context.Context, versionID int64, rows []domaincashier.PriceRow) error
	CopyRows(ctx context.Context, sourceVersionID, targetVersionID int64) (int, error)

	GetCurrencySpec(ctx context.Context, currency string) (common.CurrencySpec, error)

	CreateFXSyncRun(ctx context.Context, run domaincashier.FXSyncRun) (domaincashier.FXSyncRun, error)
	GetFXSyncRun(ctx context.Context, runID int64) (domaincashier.FXSyncRun, error)
	ListFXSyncRuns(ctx context.Context, templateIDRef int64) ([]domaincashier.FXSyncRun, error)
	UpdateFXSyncRunReview(ctx context.Context, runID int64, status domaincashier.FXRunStatus, reviewer int64, reviewedAt time.Time, note string) error

	ResolveGameRowID(ctx context.Context, gameID string) (int64, error)
	GetGameCashierProfile(ctx context.Context, gameIDRef int64) (domaincashier.GameCashierProfile, error)
	UpsertGameCashierProfile(ctx context.Context, profile domaincashier.GameCashierProfile) (domaincashier.GameCashierProfile, error)
	ListGameCashierPriceOverrides(ctx context.Context, gameIDRef int64) ([]domaincashier.GameCashierPriceOverride, error)
	ReplaceGameCashierPriceOverrides(ctx context.Context, gameIDRef int64, rows []domaincashier.GameCashierPriceOverride) error
}

type TxManager interface {
	Repository() CashierTemplateRepository
	InTx(ctx context.Context, fn func(CashierTemplateRepository) error) error
}

type AuditSink = adminapp.AuditSink
type AuditEntry = adminapp.AuditEntry
