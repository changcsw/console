package common

type Environment string

const (
	EnvDevelop    Environment = "develop"
	EnvSandbox    Environment = "sandbox"
	EnvProduction Environment = "production"
)

type LoginMode string

const (
	LoginModeChannelOnly   LoginMode = "channel_only"
	LoginModeAccountSystem LoginMode = "account_system"
)

type PaymentMode string

const (
	PaymentModeChannelOnly PaymentMode = "channel_only"
	PaymentModeHybrid      PaymentMode = "hybrid"
	PaymentModeCashierOnly PaymentMode = "cashier_only"
)

type ConfigStatus string

const (
	ConfigStatusEmpty   ConfigStatus = "empty"
	ConfigStatusInvalid ConfigStatus = "invalid"
	ConfigStatusValid   ConfigStatus = "valid"
)

type OverrideMode string

const (
	OverrideModeDefault  OverrideMode = "default"
	OverrideModeOverride OverrideMode = "override"
)

type FXSyncMode string

const (
	FXSyncModeManualConfirm FXSyncMode = "manual_confirm"
	FXSyncModeAutoApply     FXSyncMode = "auto_apply"
)

// AdminUserStatus 管理员状态（00 §3.1，默认 active）。
type AdminUserStatus string

const (
	AdminUserStatusActive   AdminUserStatus = "active"
	AdminUserStatusDisabled AdminUserStatus = "disabled"
)

// IdentityType 管理员身份类型（00 §3.1，无默认）。
type IdentityType string

const (
	IdentityTypePassword IdentityType = "password"
	IdentityTypeFeishu   IdentityType = "feishu"
)

// GameStatus 游戏状态（00 §3.1，默认 draft）。
type GameStatus string

const (
	GameStatusDraft    GameStatus = "draft"
	GameStatusActive   GameStatus = "active"
	GameStatusDisabled GameStatus = "disabled"
)

// LegalScopeType 法务链接作用域（00 §3.1，默认 default）。
type LegalScopeType string

const (
	LegalScopeDefault LegalScopeType = "default"
	LegalScopeMarket  LegalScopeType = "market"
	LegalScopeLocale  LegalScopeType = "locale"
)
