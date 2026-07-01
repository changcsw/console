package payment

import (
	"context"
	"net/http"
	"time"

	adminapp "github.com/csw/console/services/admin-api/internal/app/admin"
	"github.com/csw/console/services/admin-api/internal/domain/accountauth"
	domainpayment "github.com/csw/console/services/admin-api/internal/domain/payment"
)

const maskedValue = "masked"

type Error struct {
	Status  int
	Code    string
	Message string
	Details []any
}

func (e *Error) Error() string { return e.Message }

const (
	CodeValidation   = "VALIDATION_FAILED"
	CodeConflict     = "CONFLICT"
	CodeNotFound     = "NOT_FOUND"
	CodeRouteConflict = "ROUTE_CONFLICT"
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

func routeConflictErr(conflict *domainpayment.RouteConflictError) *Error {
	detail := map[string]any{
		"kind":       string(conflict.Kind),
		"leftIndex":  conflict.LeftIndex,
		"rightIndex": conflict.RightIndex,
	}
	if conflict.LeftID != 0 {
		detail["leftRouteId"] = conflict.LeftID
	}
	if conflict.RightID != 0 {
		detail["rightRouteId"] = conflict.RightID
	}
	return &Error{
		Status:  http.StatusConflict,
		Code:    CodeRouteConflict,
		Message: "支付路由冲突",
		Details: []any{detail},
	}
}

type ListFilter struct {
	Page      int
	PageSize  int
	Enabled   *bool
	Type      string
	Kind      string
	ProviderID string
	SubjectID  string
}

type PayWayDTO struct {
	PayWayID   string `json:"payWayId"`
	PayWayName string `json:"payWayName"`
	PayWayType string `json:"payWayType"`
	Enabled    bool   `json:"enabled"`
	Sort       int    `json:"sort"`
}

type ProviderDTO struct {
	ProviderID   string `json:"providerId"`
	ProviderName string `json:"providerName"`
	ProviderKind string `json:"providerKind"`
	Enabled      bool   `json:"enabled"`
	Sort         int    `json:"sort"`
}

type BillingSubjectDTO struct {
	SubjectID       string `json:"subjectId"`
	SubjectName     string `json:"subjectName"`
	LegalEntityName string `json:"legalEntityName"`
	Enabled         bool   `json:"enabled"`
}

type MerchantAccountDTO struct {
	MerchantAccountID string         `json:"merchantAccountId"`
	ProviderID        string         `json:"providerId"`
	SubjectID         string         `json:"subjectId"`
	MerchantID        string         `json:"merchantId"`
	MerchantName      string         `json:"merchantName"`
	ConfigJSON        map[string]any `json:"configJson"`
	Secret            string         `json:"secret"`
	Enabled           bool           `json:"enabled"`
}

type RouteSelectorDTO struct {
	PackageCode *string `json:"packageCode"`
	ChannelID   *string `json:"channelId"`
	MarketCode  string  `json:"marketCode"`
	CountryCode string  `json:"countryCode"`
	Currency    string  `json:"currency"`
}

type RouteItemDTO struct {
	ID                int64            `json:"id"`
	Selector          RouteSelectorDTO `json:"selector"`
	ProviderID        string           `json:"providerId"`
	MerchantAccountID string           `json:"merchantAccountId"`
	Priority          int              `json:"priority"`
	Enabled           bool             `json:"enabled"`
	// I4: 引用对象启用标志 + 汇总位，供前端「引用对象已禁用」行内标红。
	HasDisabledReference   bool `json:"hasDisabledReference"`
	PayWayEnabled          bool `json:"payWayEnabled"`
	ProviderEnabled        bool `json:"providerEnabled"`
	MerchantAccountEnabled bool `json:"merchantAccountEnabled"`
	ChannelEnabled         bool `json:"channelEnabled"`
	PackageEnabled         bool `json:"packageEnabled"`
}

type RouteGroupDTO struct {
	PayWayID   string         `json:"payWayId"`
	PayWayName string         `json:"payWayName"`
	PayWayType string         `json:"payWayType"`
	Routes     []RouteItemDTO `json:"routes"`
}

type GameRoutesDTO struct {
	GameID string          `json:"gameId"`
	Env    string          `json:"env"`
	Groups []RouteGroupDTO `json:"groups"`
}

type CreateBillingSubjectCommand struct {
	SubjectID       string
	SubjectName     string
	LegalEntityName string
	Enabled         bool
}

type CreateMerchantAccountCommand struct {
	MerchantAccountID string
	ProviderID        string
	SubjectID         string
	MerchantID        string
	MerchantName      string
	ConfigJSON        map[string]any
	Secrets           map[string]string
	Enabled           bool
}

type SaveRouteItem struct {
	Package           *string
	Channel           *string
	Market            *string
	Country           *string
	Currency          *string
	PayWayID          string
	ProviderID        string
	MerchantAccountID string
	Priority          *int
	Enabled           *bool
}

type SaveGameRoutesCommand struct {
	Items []SaveRouteItem
}

type PaymentRouteService interface {
	ListPayWays(ctx context.Context, filter ListFilter) ([]PayWayDTO, int, error)
	ListProviders(ctx context.Context, filter ListFilter) ([]ProviderDTO, int, error)
	ListBillingSubjects(ctx context.Context, filter ListFilter) ([]BillingSubjectDTO, int, error)
	ListMerchantAccounts(ctx context.Context, filter ListFilter) ([]MerchantAccountDTO, int, error)
	GetProviderTemplate(ctx context.Context, providerID string) (accountauth.Template, error)
	GetGameRoutes(ctx context.Context, gameID string) (GameRoutesDTO, error)
	CreateBillingSubject(ctx context.Context, cmd CreateBillingSubjectCommand) (BillingSubjectDTO, error)
	CreateMerchantAccount(ctx context.Context, cmd CreateMerchantAccountCommand) (MerchantAccountDTO, error)
	SaveGameRoutes(ctx context.Context, gameID string, cmd SaveGameRoutesCommand) (GameRoutesDTO, error)
	ResolveRoute(ctx context.Context, gameID string, input domainpayment.MatchInput) (domainpayment.RouteTarget, error)
}

type TxManager interface {
	Repository() Repository
	InTx(ctx context.Context, fn func(Repository) error) error
}

type Repository interface {
	ListPayWays(ctx context.Context, filter ListFilter) ([]PayWayDTO, int, error)
	ListProviders(ctx context.Context, filter ListFilter) ([]ProviderDTO, int, error)
	ListBillingSubjects(ctx context.Context, filter ListFilter) ([]BillingSubjectDTO, int, error)
	CreateBillingSubject(ctx context.Context, in BillingSubjectRecord) (BillingSubjectDTO, error)
	ListMerchantAccounts(ctx context.Context, filter ListFilter) ([]MerchantAccountDTO, int, error)
	CreateMerchantAccount(ctx context.Context, in MerchantAccountRecord) (MerchantAccountDTO, error)
	GetLatestProviderTemplate(ctx context.Context, providerRowID int64) (accountauth.Template, error)

	ResolveGameRowID(ctx context.Context, gameID string) (int64, error)
	ResolvePayWay(ctx context.Context, payWayID string) (PayWayRef, error)
	ResolveProvider(ctx context.Context, providerID string) (ProviderRef, error)
	ResolveSubject(ctx context.Context, subjectID string) (SubjectRef, error)
	ResolveMerchantAccount(ctx context.Context, merchantAccountID string) (MerchantAccountRef, error)
	ResolveChannel(ctx context.Context, channelID string) (int64, bool, error)
	ResolvePackage(ctx context.Context, gameRowID int64, packageCode string) (int64, bool, error)

	ReplaceGameRoutes(ctx context.Context, gameRowID int64, routes []RouteRecord) error
	ListGameRoutes(ctx context.Context, gameID string) ([]GameRouteRecord, error)
	ListEnabledRoutes(ctx context.Context, gameID, payWayID string) ([]ResolvedRouteRecord, error)
}

type CryptoService interface {
	Encrypt(plain string) (string, error)
}

type BillingSubjectRecord struct {
	SubjectID       string
	SubjectName     string
	LegalEntityName string
	Enabled         bool
}

type MerchantAccountRecord struct {
	MerchantAccountID string
	ProviderRowID     int64
	SubjectRowID      int64
	ProviderID        string
	SubjectID         string
	MerchantID        string
	MerchantName      string
	ConfigJSON        map[string]any
	SecretCiphertext  string
	Enabled           bool
}

type RouteRecord struct {
	MarketCode           string
	CountryCode          string
	Currency             string
	ChannelIDRef         *int64
	PackageIDRef         *int64
	PayWayIDRef          int64
	ProviderIDRef        int64
	MerchantAccountIDRef int64
	Priority             int
	Enabled              bool
}

type PayWayRef struct {
	RowID   int64
	PayWayID string
	Enabled bool
}

type ProviderRef struct {
	RowID      int64
	ProviderID string
	Enabled    bool
}

type SubjectRef struct {
	RowID     int64
	SubjectID string
	Enabled   bool
}

type MerchantAccountRef struct {
	RowID             int64
	MerchantAccountID string
	ProviderID        string
	Enabled           bool
}

type GameRouteRecord struct {
	ID                int64
	GameID            string
	PayWayID          string
	PayWayName        string
	PayWayType        string
	PackageCode       string
	ChannelID         string
	MarketCode        string
	CountryCode       string
	Currency          string
	ProviderID        string
	MerchantAccountID string
	Priority          int
	Enabled           bool
	// I4: 各引用对象 enabled 标志（通配即无引用时约定为 true）。
	PayWayEnabled   bool
	ProviderEnabled bool
	MerchantEnabled bool
	ChannelEnabled  bool
	PackageEnabled  bool
}

type ResolvedRouteRecord struct {
	ID                int64
	PackageCode       string
	ChannelID         string
	MarketCode        string
	CountryCode       string
	Currency          string
	PayWayID          string
	ProviderID        string
	MerchantAccountID string
	Priority          int
	Enabled           bool
	ProviderEnabled   bool
	MerchantEnabled   bool
}

type AuditSink = adminapp.AuditSink
type AuditEntry = adminapp.AuditEntry

type nowFunc func() time.Time
