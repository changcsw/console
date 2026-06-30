package dto

import "time"

// UpsertChannelLoginConfigCmd PUT /game-channels/{gameChannelId}/login-config。
type UpsertChannelLoginConfigCmd struct {
	GameChannelID   int64
	Enabled         *bool
	ConfigJSON      map[string]any
	TemplateVersion string
}

// ChannelLoginValidationDetail PUT 校验明细 details[]。
type ChannelLoginValidationDetail struct {
	Field   string `json:"field"`
	Rule    string `json:"rule"`
	Message string `json:"message"`
}

// ChannelLoginTemplateView 登录模板四件套响应。
type ChannelLoginTemplateView struct {
	TemplateVersion string         `json:"templateVersion"`
	FormSchemaJSON  []any          `json:"formSchemaJson"`
	SecretFields    []string       `json:"secretFieldsJson"`
	FileFields      []any          `json:"fileFieldsJson"`
	ValidationRules map[string]any `json:"validationRulesJson"`
}

// ChannelLoginConfigView 渠道登录配置响应。
type ChannelLoginConfigView struct {
	Enabled          bool           `json:"enabled"`
	ConfigJSON       map[string]any `json:"configJson"`
	ConfigStatus     string         `json:"configStatus"`
	LastCheckAt      *time.Time     `json:"lastCheckAt"`
	LastCheckMessage string         `json:"lastCheckMessage"`
}

// ChannelLoginView GET/PUT 响应 data。
type ChannelLoginView struct {
	GameChannelID int64                    `json:"gameChannelId"`
	Environment   string                   `json:"env"`
	ChannelID     string                   `json:"channelId"`
	MarketCode    string                   `json:"marketCode"`
	LoginMode     string                   `json:"loginMode"`
	LoginLocked   bool                     `json:"loginLocked"`
	Config        ChannelLoginConfigView   `json:"config"`
	Template      ChannelLoginTemplateView `json:"template"`
}
