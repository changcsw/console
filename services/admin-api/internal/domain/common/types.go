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

