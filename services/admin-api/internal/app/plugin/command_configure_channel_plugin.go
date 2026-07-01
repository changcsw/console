package plugin

import (
	"context"
	"strings"

	"github.com/csw/console/services/admin-api/internal/app/dto"
	domainplugin "github.com/csw/console/services/admin-api/internal/domain/plugin"
)

// ConfigureChannelPlugin 勾选/配置渠道实例插件并重算 config_status。
func (s *Service) ConfigureChannelPlugin(ctx context.Context, cmd dto.ConfigureChannelPluginCmd) (dto.ChannelPluginItemView, error) {
	zero := dto.ChannelPluginItemView{}
	pluginID := strings.TrimSpace(cmd.PluginID)
	if cmd.GameChannelID <= 0 || pluginID == "" {
		return zero, validationErr("gameChannelId 或 pluginId 非法")
	}
	config := cloneMap(cmd.Config)
	var out dto.ChannelPluginItemView
	err := s.tx.InTx(ctx, func(repos Repositories) error {
		gc, err := repos.Game.GetGameChannel(ctx, cmd.GameChannelID)
		if err != nil {
			return mapLoadErr(err, "game channel not found")
		}
		allowed, err := repos.Features.ListByChannel(ctx, gc.ChannelID)
		if err != nil {
			return err
		}
		var meta *FeaturePluginMeta
		for i := range allowed {
			if allowed[i].PluginID == pluginID {
				meta = &allowed[i]
				break
			}
		}
		if meta == nil {
			return validationErr("插件不在渠道允许集合")
		}
		if !domainplugin.ValidatePluginRegionCompatibility(gc.Market, meta.Region) {
			return incompatibleErr("插件与渠道 market 不兼容")
		}
		tpl, err := repos.Features.GetLatestTemplate(ctx, meta.ID)
		if err != nil {
			return err
		}
		if tpl == nil {
			return validationErr("插件模板不存在")
		}

		old, err := repos.Game.GetByGameChannelAndPlugin(ctx, cmd.GameChannelID, meta.ID)
		if err != nil {
			return err
		}
		enabled := boolOr(cmd.Enabled, meta.DefaultEnabled)
		if old != nil {
			if cmd.Enabled == nil {
				enabled = old.Enabled
			}
		}
		if meta.Locked {
			return validationErr("插件已锁定，不可修改")
		}
		if meta.Required && !meta.Selectable && !enabled {
			return validationErr("必接插件不可取消勾选")
		}
		status, msg := domainplugin.ResolvePluginConfigStatus(enabled, templateFrom(tpl), config)
		var oldConfig map[string]any
		if old != nil {
			oldConfig = old.ConfigJSON
		}
		stored, err := s.encryptSecrets(config, tpl.SecretFields, oldConfig)
		if err != nil {
			return err
		}
		cfg := GameChannelPluginConfig{
			GameChannelIDRef: cmd.GameChannelID,
			PluginIDRef:      meta.ID,
			Enabled:          enabled,
			ConfigJSON:       stored,
			ConfigStatus:     string(status),
			LastCheckMessage: msg,
		}
		if old != nil {
			cfg.ID = old.ID
		}
		saved, err := repos.Game.Upsert(ctx, cfg)
		if err != nil {
			return mapWriteErr(err, "plugin config conflict")
		}
		out = makeConfigView(saved, *meta, tpl, gc)
		return nil
	})
	if err != nil {
		return zero, err
	}
	_ = s.writeAudit(ctx, "plugin.configure", "game_channel_plugin_config", out.ID, map[string]any{
		"gameChannelId": cmd.GameChannelID,
		"pluginId":      pluginID,
		"configStatus":  out.ConfigStatus,
		"env":           string(s.env),
	})
	return out, nil
}

// PatchChannelPlugin 修改已存在的渠道实例插件配置。
func (s *Service) PatchChannelPlugin(ctx context.Context, cmd dto.PatchChannelPluginCmd) (dto.ChannelPluginItemView, error) {
	zero := dto.ChannelPluginItemView{}
	if cmd.ID <= 0 {
		return zero, validationErr("id 非法")
	}
	config := cloneMap(cmd.Config)
	var out dto.ChannelPluginItemView
	err := s.tx.InTx(ctx, func(repos Repositories) error {
		old, err := repos.Game.GetByID(ctx, cmd.ID)
		if err != nil {
			return mapLoadErr(err, "plugin config not found")
		}
		gc, err := repos.Game.GetGameChannel(ctx, old.GameChannelIDRef)
		if err != nil {
			return mapLoadErr(err, "game channel not found")
		}
		metaList, err := repos.Features.ListByChannel(ctx, gc.ChannelID)
		if err != nil {
			return err
		}
		var meta *FeaturePluginMeta
		for i := range metaList {
			if metaList[i].ID == old.PluginIDRef {
				meta = &metaList[i]
				break
			}
		}
		if meta == nil {
			return validationErr("插件不在渠道允许集合")
		}
		if !domainplugin.ValidatePluginRegionCompatibility(gc.Market, meta.Region) {
			return incompatibleErr("插件与渠道 market 不兼容")
		}
		tpl, err := repos.Features.GetLatestTemplate(ctx, old.PluginIDRef)
		if err != nil {
			return err
		}
		if tpl == nil {
			return validationErr("插件模板不存在")
		}
		if meta.Locked {
			return validationErr("插件已锁定，不可修改")
		}
		enabled := old.Enabled
		if cmd.Enabled != nil {
			enabled = *cmd.Enabled
		}
		if meta.Required && !meta.Selectable && !enabled {
			return validationErr("必接插件不可取消勾选")
		}
		if len(config) == 0 {
			config = cloneMap(old.ConfigJSON)
		}
		status, msg := domainplugin.ResolvePluginConfigStatus(enabled, templateFrom(tpl), config)
		stored, err := s.encryptSecrets(config, tpl.SecretFields, old.ConfigJSON)
		if err != nil {
			return err
		}
		old.Enabled = enabled
		old.ConfigJSON = stored
		old.ConfigStatus = string(status)
		old.LastCheckMessage = msg
		saved, err := repos.Game.Upsert(ctx, old)
		if err != nil {
			return mapWriteErr(err, "plugin config conflict")
		}
		out = makeConfigView(saved, *meta, tpl, gc)
		return nil
	})
	if err != nil {
		return zero, err
	}
	_ = s.writeAudit(ctx, "plugin.configure", "game_channel_plugin_config", out.ID, map[string]any{
		"pluginConfigId": out.ID,
		"configStatus":   out.ConfigStatus,
		"env":            string(s.env),
	})
	return out, nil
}
