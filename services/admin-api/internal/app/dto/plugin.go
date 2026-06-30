package dto

import (
	"time"

	domainplugin "github.com/csw/console/services/admin-api/internal/domain/plugin"
)

type ConfigureChannelPluginCmd struct {
	GameChannelID int64
	PluginID      string
	Enabled       *bool
	Config        map[string]any
}

type PatchChannelPluginCmd struct {
	ID      int64
	Enabled *bool
	Config  map[string]any
}

type OverridePackagePluginCmd struct {
	PackageID            int64
	PluginID             string
	InheritChannelConfig *bool
	Enabled              *bool
	Config               map[string]any
}

type FeaturePluginTemplateView struct {
	TemplateVersion     string                                 `json:"templateVersion"`
	FormSchemaJSON      []domainplugin.TemplateField           `json:"formSchemaJson"`
	SecretFieldsJSON    []string                               `json:"secretFieldsJson"`
	FileFieldsJSON      []domainplugin.FileField               `json:"fileFieldsJson"`
	ValidationRulesJSON map[string]domainplugin.ValidationRule `json:"validationRulesJson"`
}

type ChannelPluginItemView struct {
	ID                      int64                     `json:"id"`
	PluginID                string                    `json:"pluginId"`
	PluginName              string                    `json:"pluginName"`
	Region                  string                    `json:"region"`
	Required                bool                      `json:"required"`
	Selectable              bool                      `json:"selectable"`
	Locked                  bool                      `json:"locked"`
	Enabled                 bool                      `json:"enabled"`
	ConfigStatus            string                    `json:"configStatus"`
	LastCheckMessage        string                    `json:"lastCheckMessage"`
	IncludedInRuntimeConfig bool                      `json:"includedInRuntimeConfig"`
	IncludedInSnapshot      bool                      `json:"includedInSnapshot,omitempty"`
	IncludedInSync          bool                      `json:"includedInSync,omitempty"`
	ConfigJSON              map[string]any            `json:"configJson"`
	LastCheckAt             *time.Time                `json:"lastCheckAt"`
	Template                FeaturePluginTemplateView `json:"template"`
	UpdatedAt               *time.Time                `json:"updatedAt,omitempty"`
}

type ChannelPluginListView struct {
	Items                  []ChannelPluginItemView `json:"items"`
	MissingRequiredPlugins []string                `json:"missingRequiredPlugins"`
}

type ChannelPluginConfigView = ChannelPluginItemView

type PackagePluginItemView struct {
	ID                      int64                     `json:"id"`
	PackageID               int64                     `json:"packageId"`
	PluginID                string                    `json:"pluginId"`
	PluginName              string                    `json:"pluginName,omitempty"`
	Region                  string                    `json:"region,omitempty"`
	Required                bool                      `json:"required,omitempty"`
	Selectable              bool                      `json:"selectable,omitempty"`
	Locked                  bool                      `json:"locked,omitempty"`
	InheritChannelConfig    bool                      `json:"inheritChannelConfig"`
	Enabled                 bool                      `json:"enabled"`
	ConfigJSON              map[string]any            `json:"configJson"`
	ConfigStatus            string                    `json:"configStatus"`
	LastCheckMessage        string                    `json:"lastCheckMessage"`
	IncludedInRuntimeConfig bool                      `json:"includedInRuntimeConfig"`
	LastCheckAt             *time.Time                `json:"lastCheckAt"`
	Template                FeaturePluginTemplateView `json:"template,omitempty"`
}
