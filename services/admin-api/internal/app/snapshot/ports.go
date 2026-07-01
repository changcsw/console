package snapshot

import (
	"context"
	"errors"
	"net/http"
	"time"

	adminapp "github.com/csw/console/services/admin-api/internal/app/admin"
	paymentapp "github.com/csw/console/services/admin-api/internal/app/payment"
	domainpayment "github.com/csw/console/services/admin-api/internal/domain/payment"
	domainsnapshot "github.com/csw/console/services/admin-api/internal/domain/snapshot"
)

var (
	ErrValidation = errors.New("validation failed")
	ErrNotFound   = errors.New("not found")
	ErrConflict   = errors.New("conflict")
)

const (
	CodeValidation          = "VALIDATION_FAILED"
	CodeNotFound            = "NOT_FOUND"
	CodeConflict            = "CONFLICT"
	CodeVersionStateInvalid = "VERSION_STATE_INVALID"
)

type Error struct {
	Status  int
	Code    string
	Message string
	Details []any
}

func (e *Error) Error() string { return e.Message }

func validationErr(msg string, details ...any) *Error {
	return &Error{Status: http.StatusBadRequest, Code: CodeValidation, Message: msg, Details: details}
}

func notFoundErr(msg string) *Error {
	return &Error{Status: http.StatusNotFound, Code: CodeNotFound, Message: msg, Details: []any{}}
}

func conflictErr(msg string) *Error {
	return &Error{Status: http.StatusConflict, Code: CodeConflict, Message: msg, Details: []any{}}
}

func versionStateErr(msg string) *Error {
	return &Error{Status: http.StatusConflict, Code: CodeVersionStateInvalid, Message: msg, Details: []any{}}
}

type ListFilter struct {
	Page     int
	PageSize int
}

type GenerateResult struct {
	ID            int64     `json:"id"`
	ConfigVersion string    `json:"configVersion"`
	FileHash      string    `json:"fileHash"`
	Status        string    `json:"status"`
	GeneratedAt   time.Time `json:"generatedAt"`
}

type SnapshotItem struct {
	ID            int64      `json:"id"`
	ConfigVersion string     `json:"configVersion"`
	Status        string     `json:"status"`
	FileHash      string     `json:"fileHash"`
	GeneratedAt   time.Time  `json:"generatedAt"`
	PublishedAt   *time.Time `json:"publishedAt"`
}

type SnapshotList struct {
	Items    []SnapshotItem `json:"items"`
	Page     int            `json:"page"`
	PageSize int            `json:"pageSize"`
	Total    int            `json:"total"`
}

type DownloadResult struct {
	FileName string
	Body     []byte
}

type Service interface {
	Generate(ctx context.Context, gameID string) (GenerateResult, error)
	List(ctx context.Context, gameID string, filter ListFilter) (SnapshotList, error)
	Publish(ctx context.Context, snapshotID int64) (SnapshotItem, error)
	Download(ctx context.Context, snapshotID int64) (DownloadResult, error)
}

type TxManager interface {
	Repository() Repository
	InTx(ctx context.Context, fn func(Repository) error) error
}

type Repository interface {
	ResolveGameRowID(ctx context.Context, gameID string) (int64, error)
	LoadValidData(ctx context.Context, gameIDRef int64, gameID string, generatedAt time.Time) (domainsnapshot.ValidDataView, []string, error)
	CreateSnapshot(ctx context.Context, in CreateSnapshotInput) (domainsnapshot.ConfigSnapshot, error)
	ListSnapshots(ctx context.Context, gameID string, filter ListFilter) ([]domainsnapshot.ConfigSnapshot, int, error)
	GetSnapshot(ctx context.Context, snapshotID int64) (domainsnapshot.ConfigSnapshot, error)
	PublishSnapshot(ctx context.Context, snapshotID int64, publishedAt time.Time) (domainsnapshot.ConfigSnapshot, error)
}

type CreateSnapshotInput struct {
	GameIDRef           int64
	ConfigSchemaVersion string
	ConfigVersion       string
	ConfigJSON          map[string]any
	FileName            string
	FileHash            string
	StorageKey          string
	GeneratedAt         time.Time
}

type PaymentResolver interface {
	ResolveRoute(ctx context.Context, gameID string, input domainpayment.MatchInput) (domainpayment.RouteTarget, error)
}

type AuditSink = adminapp.AuditSink
type AuditEntry = adminapp.AuditEntry

func isNotFoundRoute(err error) bool {
	if err == nil {
		return false
	}
	var appErr *paymentapp.Error
	if errors.As(err, &appErr) && appErr.Code == paymentapp.CodeNotFound {
		return true
	}
	return errors.Is(err, domainpayment.ErrRouteNotFound)
}
