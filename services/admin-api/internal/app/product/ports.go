package product

import (
	"context"
	"net/http"
	"time"

	adminapp "github.com/csw/console/services/admin-api/internal/app/admin"
	"github.com/csw/console/services/admin-api/internal/domain/accountauth"
	"github.com/csw/console/services/admin-api/internal/domain/common"
	domainproduct "github.com/csw/console/services/admin-api/internal/domain/product"
)

type Error struct {
	Status  int
	Code    string
	Message string
	Details []any
}

func (e *Error) Error() string { return e.Message }

const (
	codeValidation = "VALIDATION_FAILED"
	codeConflict   = "CONFLICT"
	codeNotFound   = "NOT_FOUND"
	codeCurrency   = "CURRENCY_NOT_SUPPORTED"
	codeForbidden  = "FORBIDDEN"
	maskedValue    = "masked"
)

func validationErr(msg string, details ...any) *Error {
	if details == nil {
		details = []any{}
	}
	return &Error{Status: http.StatusBadRequest, Code: codeValidation, Message: msg, Details: details}
}

func conflictErr(msg string) *Error {
	return &Error{Status: http.StatusConflict, Code: codeConflict, Message: msg, Details: []any{}}
}

func notFoundErr(msg string) *Error {
	return &Error{Status: http.StatusNotFound, Code: codeNotFound, Message: msg, Details: []any{}}
}

func currencyErr(msg string) *Error {
	return &Error{Status: http.StatusBadRequest, Code: codeCurrency, Message: msg, Details: []any{}}
}

func forbiddenErr(msg string) *Error {
	return &Error{Status: http.StatusForbidden, Code: codeForbidden, Message: msg, Details: []any{}}
}

func fieldDetail(field, reason string) any {
	return map[string]string{"field": field, "reason": reason}
}

type ProductRepository interface {
	ListByGame(ctx context.Context, gameID string, keyword string, enabled *bool, page, pageSize int, sort string) ([]domainproduct.Product, int, error)
	Create(ctx context.Context, item domainproduct.Product) (domainproduct.Product, error)
	GetByGameAndProductID(ctx context.Context, gameID, productID string) (domainproduct.Product, error)
	Update(ctx context.Context, gameID, productID string, patch ProductPatch) (domainproduct.Product, error)
	ListByIDs(ctx context.Context, gameID string, productIDs []string) ([]domainproduct.Product, error)
}

type ProductPatch struct {
	ProductName     *string
	BaseAmountMinor *int64
	BaseCurrency    *string
	PriceID         *string
	Enabled         *bool
}

type ChannelProductRepository interface {
	ListByPackage(ctx context.Context, packageID int64) ([]domainproduct.ChannelProduct, error)
	ReplaceByPackage(ctx context.Context, packageID int64, items []domainproduct.ChannelProduct) error
}

type ChannelPackageRepository interface {
	GetPackageGameAndChannel(ctx context.Context, packageID int64) (gameID string, packageCode string, channelID string, gameChannelID int64, err error)
	BelongsToGame(ctx context.Context, packageID int64, gameID string) (bool, error)
}

type CurrencySpecRepository interface {
	GetByCode(ctx context.Context, currencyCode string) (common.CurrencySpec, error)
}

type GameChannelIapConfigRepository interface {
	GetByGameChannelID(ctx context.Context, gameChannelID int64) (domainproduct.IAPConfig, bool, error)
	UpsertByGameChannelID(ctx context.Context, gameChannelID int64, cfg domainproduct.IAPConfig) (domainproduct.IAPConfig, error)
	GetChannelInfo(ctx context.Context, gameChannelID int64) (channelID string, err error)
}

type ChannelPackageIapOverrideRepository interface {
	GetByPackageID(ctx context.Context, packageID int64) (domainproduct.IAPConfig, bool, error)
	UpsertByPackageID(ctx context.Context, packageID int64, cfg domainproduct.IAPConfig) (domainproduct.IAPConfig, error)
}

type ChannelIapTemplateRepository interface {
	GetLatestEnabledByChannelID(ctx context.Context, channelID string) (accountauth.Template, error)
}

type CryptoService interface {
	Encrypt(plain string) (string, error)
}

type FileService interface {
	NormalizeReference(value string) (string, error)
}

type Repositories struct {
	Products            ProductRepository
	ChannelProducts     ChannelProductRepository
	Packages            ChannelPackageRepository
	CurrencySpecs       CurrencySpecRepository
	GameChannelIAP      GameChannelIapConfigRepository
	PackageIAPOverrides ChannelPackageIapOverrideRepository
	ChannelIAPTemplates ChannelIapTemplateRepository
}

type TxManager interface {
	Repositories() Repositories
	InTx(ctx context.Context, fn func(Repositories) error) error
}

type AuditSink = adminapp.AuditSink
type AuditEntry = adminapp.AuditEntry

type nowFunc func() time.Time
