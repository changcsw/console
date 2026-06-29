package accountauth

import (
	"context"
	"net/http"
	"time"

	adminapp "github.com/csw/console/services/admin-api/internal/app/admin"
	"github.com/csw/console/services/admin-api/internal/domain/accountauth"
	"github.com/csw/console/services/admin-api/internal/domain/common"
)

type Error struct {
	Status  int
	Code    string
	Message string
	Details []any
}

func (e *Error) Error() string { return e.Message }

// 模块错误码（00 §7.4 全局 + 本模块私有）。
// 说明：账号认证配置写为「整体替换」，不做乐观并发检测（last-writer-wins），
// 因此本模块不暴露业务级 CONFLICT；故不声明 codeConflict（见 handoff CONFLICT 选择）。
const (
	codeValidation         = "VALIDATION_FAILED"
	codeNotFound           = "NOT_FOUND"
	codeTypeNotAllowed     = "ACCOUNT_AUTH_TYPE_NOT_ALLOWED"
	codeTemplateNotFound   = "ACCOUNT_AUTH_TEMPLATE_NOT_FOUND"
	codeEncryptionRequired = "ACCOUNT_AUTH_SECRET_ENCRYPT_FAILED"
)

func validationErr(msg string, details ...any) *Error {
	if details == nil {
		details = []any{}
	}
	return &Error{Status: http.StatusBadRequest, Code: codeValidation, Message: msg, Details: details}
}

func notFoundErr(msg string) *Error {
	return &Error{Status: http.StatusNotFound, Code: codeNotFound, Message: msg, Details: []any{}}
}

func typeNotAllowedErr(msg string) *Error {
	return &Error{Status: http.StatusBadRequest, Code: codeTypeNotAllowed, Message: msg, Details: []any{}}
}

func templateNotFoundErr(msg string) *Error {
	return &Error{Status: http.StatusBadRequest, Code: codeTemplateNotFound, Message: msg, Details: []any{}}
}

func encryptionErr(msg string) *Error {
	return &Error{Status: http.StatusInternalServerError, Code: codeEncryptionRequired, Message: msg, Details: []any{}}
}

func fieldDetail(field, reason string) any {
	return map[string]string{"field": field, "reason": reason}
}

type TypeCatalogItem struct {
	AuthTypeIDRef int64
	AuthTypeID    string
	AuthTypeName  string
	Enabled       bool
	Sort          int
	Template      accountauth.Template
}

type ChannelTypePolicy struct {
	AuthTypeID     string
	DefaultEnabled bool
	Locked         bool
}

type GameAllowedType struct {
	AuthTypeIDRef  int64
	AuthTypeID     string
	DefaultEnabled bool
	Locked         bool
	Template       accountauth.Template
}

type GameConfigItem struct {
	AuthTypeIDRef    int64
	Enabled          bool
	ConfigJSON       map[string]any
	ConfigStatus     common.ConfigStatus
	LastCheckAt      *time.Time
	LastCheckMessage string
}

type GameConfigUpsert struct {
	AuthTypeIDRef    int64
	Enabled          bool
	ConfigJSON       map[string]any
	ConfigStatus     common.ConfigStatus
	LastCheckAt      *time.Time
	LastCheckMessage string
}

type Repository interface {
	ListTypeCatalog(ctx context.Context) ([]TypeCatalogItem, error)
	ListChannelPolicies(ctx context.Context, channelID string) ([]ChannelTypePolicy, error)
	ResolveGameRowID(ctx context.Context, gameID string) (int64, error)
	ListAllowedTypesByGame(ctx context.Context, gameIDRef int64) ([]GameAllowedType, error)
	ListGameConfigs(ctx context.Context, gameIDRef int64) ([]GameConfigItem, error)
	ReplaceGameConfigs(ctx context.Context, gameIDRef int64, items []GameConfigUpsert) error
}

type TxManager interface {
	Repository() Repository
	InTx(ctx context.Context, fn func(Repository) error) error
}

type Cipher interface {
	Encrypt(plain string) (string, error)
}

type AuditSink = adminapp.AuditSink
type AuditEntry = adminapp.AuditEntry
