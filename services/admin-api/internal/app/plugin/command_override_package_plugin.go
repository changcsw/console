package plugin

import (
	"context"
	"strings"

	"github.com/csw/console/services/admin-api/internal/app/dto"
	domainplugin "github.com/csw/console/services/admin-api/internal/domain/plugin"
)

// OverridePackagePlugin 渠道包级插件覆盖（继承/自定义，仅存差异）。
func (s *Service) OverridePackagePlugin(ctx context.Context, cmd dto.OverridePackagePluginCmd) (dto.PackagePluginItemView, error) {
	zero := dto.PackagePluginItemView{}
	if cmd.PackageID <= 0 || strings.TrimSpace(cmd.PluginID) == "" {
		return zero, validationErr("packageId 或 pluginId 非法")
	}
	var out dto.PackagePluginItemView
	err := s.tx.InTx(ctx, func(repos Repositories) error {
		pkg, err := repos.Packages.GetPackage(ctx, cmd.PackageID)
		if err != nil {
			return mapLoadErr(err, "package not found")
		}
		meta, err := repos.Features.GetByPluginID(ctx, cmd.PluginID)
		if err != nil {
			return mapLoadErr(err, "plugin not found")
		}
		if !domainplugin.ValidatePluginRegionCompatibility(pkg.MarketCode, meta.Region) {
			return incompatibleErr("插件与渠道 market 不兼容")
		}
		tpl, err := repos.Features.GetLatestTemplate(ctx, meta.ID)
		if err != nil {
			return err
		}
		if tpl == nil {
			return validationErr("插件模板不存在")
		}

		exist, err := repos.Packages.GetByPackageAndPlugin(ctx, cmd.PackageID, meta.ID)
		if err != nil {
			return err
		}
		inherit := boolOr(cmd.InheritChannelConfig, true)
		enabled := boolOr(cmd.Enabled, false)
		if exist != nil {
			if cmd.InheritChannelConfig == nil {
				inherit = exist.InheritChannelConfig
			}
			if cmd.Enabled == nil {
				enabled = exist.Enabled
			}
		}

		status := "empty"
		msg := ""
		config := cloneMap(cmd.Config)
		if inherit {
			config = map[string]any{}
			enabled = false
		} else {
			st, m := domainplugin.ResolvePluginConfigStatus(enabled, templateFrom(tpl), config)
			status, msg = string(st), m
		}
		var oldConfig map[string]any
		if exist != nil {
			oldConfig = exist.ConfigJSON
		}
		stored, err := s.encryptSecrets(config, tpl.SecretFields, oldConfig)
		if err != nil {
			return err
		}
		row := ChannelPackagePluginOverride{
			PackageIDRef:         cmd.PackageID,
			PluginIDRef:          meta.ID,
			InheritChannelConfig: inherit,
			Enabled:              enabled,
			ConfigJSON:           stored,
			ConfigStatus:         status,
			LastCheckMessage:     msg,
		}
		if exist != nil {
			row.ID = exist.ID
		}
		saved, err := repos.Packages.Upsert(ctx, row)
		if err != nil {
			return mapWriteErr(err, "package plugin conflict")
		}
		out = dto.PackagePluginItemView{
			ID:                   saved.ID,
			PackageID:            saved.PackageIDRef,
			PluginID:             cmd.PluginID,
			PluginName:           meta.Name,
			Region:               meta.Region,
			Required:             meta.Required,
			Selectable:           meta.Selectable,
			Locked:               meta.Locked,
			InheritChannelConfig: saved.InheritChannelConfig,
			Enabled:              saved.Enabled,
			ConfigJSON:           maskSecrets(saved.ConfigJSON, tpl.SecretFields),
			ConfigStatus:         saved.ConfigStatus,
			LastCheckMessage:     saved.LastCheckMessage,
			LastCheckAt:          saved.LastCheckAt,
			Template:             templateView(tpl),
		}
		return nil
	})
	if err != nil {
		return zero, err
	}
	_ = s.writeAudit(ctx, "plugin.configure", "channel_package_plugin_override", out.ID, map[string]any{
		"packageId": cmd.PackageID,
		"pluginId":  cmd.PluginID,
	})
	return out, nil
}
