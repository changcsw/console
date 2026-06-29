package dto

import "time"

type AccountAuthTemplateView struct {
	TemplateVersion string         `json:"templateVersion"`
	FormSchema      []any          `json:"formSchema"`
	SecretFields    []string       `json:"secretFields"`
	FileFields      []any          `json:"fileFields"`
	ValidationRules map[string]any `json:"validationRules"`
}

type AccountAuthTypeView struct {
	AuthTypeID   string                  `json:"authTypeId"`
	AuthTypeName string                  `json:"authTypeName"`
	Enabled      bool                    `json:"enabled"`
	Sort         int                     `json:"sort"`
	Template     AccountAuthTemplateView `json:"template"`
}

type ChannelAccountAuthTypeView struct {
	AuthTypeID     string `json:"authTypeId"`
	DefaultEnabled bool   `json:"defaultEnabled"`
	Locked         bool   `json:"locked"`
}

type GameAccountAuthConfigView struct {
	AuthTypeID       string     `json:"authTypeId"`
	Enabled          bool       `json:"enabled"`
	ConfigJSON       any        `json:"configJson"`
	ConfigStatus     string     `json:"configStatus"`
	LastCheckAt      *time.Time `json:"lastCheckAt"`
	LastCheckMessage string     `json:"lastCheckMessage"`
}

type ReplaceGameAccountAuthConfigItem struct {
	AuthTypeID string         `json:"authTypeId"`
	Enabled    *bool          `json:"enabled"`
	ConfigJSON map[string]any `json:"configJson"`
}

type ReplaceGameAccountAuthConfigsCmd struct {
	GameID string
	Items  []ReplaceGameAccountAuthConfigItem
}
