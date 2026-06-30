package plugin

import (
	"context"
	"net/http"
	"time"

	adminapp "github.com/csw/console/services/admin-api/internal/app/admin"
	domainplugin "github.com/csw/console/services/admin-api/internal/domain/plugin"
)

type Error struct {
	Status  int
	Code    string
	Message string
	Details []any
}

func (e *Error) Error() string { return e.Message }

const (
	codeValidation   = "VALIDATION_FAILED"
	codeConflict     = "CONFLICT"
	codeNotFound     = "NOT_FOUND"
	codeIncompatible = "MARKET_CHANNEL_INCOMPATIBLE"
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

func incompatibleErr(msg string) *Error {
	return &Error{Status: http.StatusBadRequest, Code: codeIncompatible, Message: msg, Details: []any{}}
}

type GameChannelContext struct {
	ID        int64
	Market    string
	ChannelID int64
	Hidden    bool
}

type ChannelPackageContext struct {
	ID          int64
	GameChannel int64
	MarketCode  string
}

type FeaturePluginMeta struct {
	ID             int64
	PluginID       string
	Name           string
	Region         string
	Enabled        bool
	Required       bool
	Selectable     bool
	Locked         bool
	DefaultEnabled bool
}

type FeaturePluginTemplate struct {
	PluginIDRef     int64
	TemplateVersion string
	SecretFields    []string
	FormSchema      []domainplugin.TemplateField
	FileFields      []domainplugin.FileField
	ValidationRules map[string]domainplugin.ValidationRule
}

type GameChannelPluginConfig struct {
	ID               int64
	GameChannelIDRef int64
	PluginIDRef      int64
	Enabled          bool
	ConfigJSON       map[string]any
	ConfigStatus     string
	LastCheckAt      *time.Time
	LastCheckMessage string
	UpdatedAt        time.Time
}

type ChannelPackagePluginOverride struct {
	ID                   int64
	PackageIDRef         int64
	PluginIDRef          int64
	InheritChannelConfig bool
	Enabled              bool
	ConfigJSON           map[string]any
	ConfigStatus         string
	LastCheckAt          *time.Time
	LastCheckMessage     string
	UpdatedAt            time.Time
}

type FeaturePluginRepository interface {
	ListByChannel(ctx context.Context, channelIDRef int64) ([]FeaturePluginMeta, error)
	GetByPluginID(ctx context.Context, pluginID string) (FeaturePluginMeta, error)
	GetLatestTemplate(ctx context.Context, pluginIDRef int64) (*FeaturePluginTemplate, error)
}

type GameChannelPluginRepository interface {
	GetGameChannel(ctx context.Context, id int64) (GameChannelContext, error)
	GetByID(ctx context.Context, id int64) (GameChannelPluginConfig, error)
	GetByGameChannelAndPlugin(ctx context.Context, gameChannelID, pluginIDRef int64) (*GameChannelPluginConfig, error)
	ListByGameChannel(ctx context.Context, gameChannelID int64) ([]GameChannelPluginConfig, error)
	Upsert(ctx context.Context, cfg GameChannelPluginConfig) (GameChannelPluginConfig, error)
}

type ChannelPackagePluginRepository interface {
	GetPackage(ctx context.Context, id int64) (ChannelPackageContext, error)
	GetByID(ctx context.Context, id int64) (ChannelPackagePluginOverride, error)
	GetByPackageAndPlugin(ctx context.Context, packageID, pluginIDRef int64) (*ChannelPackagePluginOverride, error)
	ListByPackage(ctx context.Context, packageID int64) ([]ChannelPackagePluginOverride, error)
	Upsert(ctx context.Context, cfg ChannelPackagePluginOverride) (ChannelPackagePluginOverride, error)
}

type Repositories struct {
	Features FeaturePluginRepository
	Game     GameChannelPluginRepository
	Packages ChannelPackagePluginRepository
}

type TxManager interface {
	Repositories() Repositories
	InTx(ctx context.Context, fn func(Repositories) error) error
}

type Cipher interface {
	Encrypt(plain string) (string, error)
}

type AuditSink = adminapp.AuditSink
type AuditEntry = adminapp.AuditEntry
