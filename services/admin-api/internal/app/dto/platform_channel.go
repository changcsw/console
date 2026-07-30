package dto

import "time"

// ===== Commands（平台渠道主数据与渠道模版；系统管理员维护，与游戏无关）=====

// ListPlatformChannelsQuery 平台渠道列表查询（GET /platform/channels）。
// Keyword 模糊匹配 channel_id / channel_name；Enabled 为 nil 表示不限。
type ListPlatformChannelsQuery struct {
	Keyword     string
	Region      string
	ChannelType string
	Enabled     *bool
	Page        int
	PageSize    int
}

// CreatePlatformChannelCmd 新建平台渠道 + 策略（POST /platform/channels）。
type CreatePlatformChannelCmd struct {
	ChannelID     string
	ChannelName   string
	ChannelType   string
	Region        string
	Enabled       *bool // 缺省 true
	Sort          *int  // 缺省 0
	LoginMode     string
	PaymentMode   string
	LoginLocked   *bool
	PaymentLocked *bool
}

// UpdatePlatformChannelCmd 编辑平台渠道 + 策略（PATCH /platform/channels/{channelId}）。
// nil 表示不改。channelId / channelType / region 属身份与 market 兼容性口径，创建后不可改。
type UpdatePlatformChannelCmd struct {
	ChannelID     string
	ChannelName   *string
	Enabled       *bool
	Sort          *int
	LoginMode     *string
	PaymentMode   *string
	LoginLocked   *bool
	PaymentLocked *bool
}

// TemplateFieldInput 模版表单字段入参。
type TemplateFieldInput struct {
	Key         string                     `json:"key"`
	Label       string                     `json:"label"`
	Component   string                     `json:"component"`
	Required    bool                       `json:"required"`
	Order       int                        `json:"order"`
	Group       string                     `json:"group"`
	Scope       string                     `json:"scope"`
	Placeholder string                     `json:"placeholder"`
	Options     []TemplateFieldOptionInput `json:"options"`
}

// TemplateFieldOptionInput 下拉候选项入参。
type TemplateFieldOptionInput struct {
	Label string `json:"label"`
	Value any    `json:"value"`
}

// TemplateFileFieldInput 模版文件字段入参。
type TemplateFileFieldInput struct {
	Key       string   `json:"key"`
	Accept    []string `json:"accept"`
	MaxSizeKB *int     `json:"maxSizeKB"`
}

// TemplateRuleInput 模版校验规则入参。
type TemplateRuleInput struct {
	Required bool     `json:"required"`
	MinLen   *int     `json:"minLen"`
	MaxLen   *int     `json:"maxLen"`
	Min      *float64 `json:"min"`
	Max      *float64 `json:"max"`
	Pattern  string   `json:"pattern"`
	Format   string   `json:"format"`
	Enum     []string `json:"enum"`
}

// CreateChannelTemplateCmd 新建渠道模版版本（POST /platform/channels/{channelId}/templates）。
type CreateChannelTemplateCmd struct {
	ChannelID       string
	Kind            string
	TemplateVersion string
	FormSchema      []TemplateFieldInput
	SecretFields    []string
	FileFields      []TemplateFileFieldInput
	ValidationRules map[string]TemplateRuleInput
	Enabled         *bool // 缺省 true
}

// UpdateChannelTemplateCmd 编辑渠道模版版本（PATCH /platform/channel-templates/{kind}/{templateId}）。
// 四件套为整体替换语义：为 nil 表示该件不改。
type UpdateChannelTemplateCmd struct {
	Kind            string
	TemplateID      int64
	FormSchema      []TemplateFieldInput
	SecretFields    []string
	FileFields      []TemplateFileFieldInput
	ValidationRules map[string]TemplateRuleInput
	Enabled         *bool
}

// ===== Views =====

// PlatformChannelView 平台渠道主数据 + 策略 + 各类模版版本数。
type PlatformChannelView struct {
	ChannelID          string    `json:"channelId"`
	ChannelName        string    `json:"channelName"`
	ChannelType        string    `json:"channelType"`
	Region             string    `json:"region"`
	Enabled            bool      `json:"enabled"`
	Sort               int       `json:"sort"`
	LoginMode          string    `json:"loginMode"`
	PaymentMode        string    `json:"paymentMode"`
	LoginLocked        bool      `json:"loginLocked"`
	PaymentLocked      bool      `json:"paymentLocked"`
	LoginTemplateCount int       `json:"loginTemplateCount"`
	IAPTemplateCount   int       `json:"iapTemplateCount"`
	UpdatedAt          time.Time `json:"updatedAt"`
}

// ChannelTemplateView 渠道模版版本视图（四件套原样回传，供前端模版编辑器渲染）。
// Effective 标记该版本是否为当前生效版本（同渠道同类中 enabled 的最新 template_version）。
type ChannelTemplateView struct {
	TemplateID          int64          `json:"templateId"`
	Kind                string         `json:"kind"`
	ChannelID           string         `json:"channelId"`
	TemplateVersion     string         `json:"templateVersion"`
	FormSchemaJSON      []any          `json:"formSchemaJson"`
	SecretFieldsJSON    []string       `json:"secretFieldsJson"`
	FileFieldsJSON      []any          `json:"fileFieldsJson"`
	ValidationRulesJSON map[string]any `json:"validationRulesJson"`
	Enabled             bool           `json:"enabled"`
	Effective           bool           `json:"effective"`
	CreatedAt           time.Time      `json:"createdAt"`
	UpdatedAt           time.Time      `json:"updatedAt"`
}
