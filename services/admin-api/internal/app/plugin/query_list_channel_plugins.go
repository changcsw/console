package plugin

import (
	"context"

	"github.com/csw/console/services/admin-api/internal/app/dto"
	"github.com/csw/console/services/admin-api/internal/domain/common"
	domainplugin "github.com/csw/console/services/admin-api/internal/domain/plugin"
)

// ListChannelPlugins 列渠道实例可接插件 + 实例态 + 必接缺口。
func (s *Service) ListChannelPlugins(ctx context.Context, gameChannelID int64) (dto.ChannelPluginListView, error) {
	out := dto.ChannelPluginListView{Items: []dto.ChannelPluginItemView{}, MissingRequiredPlugins: []string{}}
	gc, err := s.tx.Repositories().Game.GetGameChannel(ctx, gameChannelID)
	if err != nil {
		return out, mapLoadErr(err, "game channel not found")
	}
	metas, err := s.tx.Repositories().Features.ListByChannel(ctx, gc.ChannelID)
	if err != nil {
		return out, err
	}
	configs, err := s.tx.Repositories().Game.ListByGameChannel(ctx, gameChannelID)
	if err != nil {
		return out, err
	}
	cfgByPlugin := make(map[int64]GameChannelPluginConfig, len(configs))
	for _, cfg := range configs {
		cfgByPlugin[cfg.PluginIDRef] = cfg
	}
	for _, meta := range metas {
		if !meta.Enabled {
			continue
		}
		if !domainplugin.ValidatePluginRegionCompatibility(gc.Market, meta.Region) {
			continue
		}
		cfg, ok := cfgByPlugin[meta.ID]
		if !ok {
			cfg = GameChannelPluginConfig{
				PluginIDRef:  meta.ID,
				Enabled:      meta.DefaultEnabled,
				ConfigStatus: string(common.ConfigStatusEmpty),
				ConfigJSON:   map[string]any{},
			}
		}
		tpl, err := s.tx.Repositories().Features.GetLatestTemplate(ctx, meta.ID)
		if err != nil {
			return out, err
		}
		out.Items = append(out.Items, makeConfigView(cfg, meta, tpl, gc))
		if meta.Required && (!cfg.Enabled || cfg.ConfigStatus != string(common.ConfigStatusValid)) {
			out.MissingRequiredPlugins = append(out.MissingRequiredPlugins, meta.PluginID)
		}
	}
	return out, nil
}

// ListPackagePlugins 列渠道包插件覆盖列表（继承/自定义）。
func (s *Service) ListPackagePlugins(ctx context.Context, packageID int64) ([]dto.PackagePluginItemView, error) {
	pkg, err := s.tx.Repositories().Packages.GetPackage(ctx, packageID)
	if err != nil {
		return nil, mapLoadErr(err, "package not found")
	}
	gc, err := s.tx.Repositories().Game.GetGameChannel(ctx, pkg.GameChannel)
	if err != nil {
		return nil, mapLoadErr(err, "game channel not found")
	}
	metas, err := s.tx.Repositories().Features.ListByChannel(ctx, gc.ChannelID)
	if err != nil {
		return nil, err
	}
	rows, err := s.tx.Repositories().Packages.ListByPackage(ctx, packageID)
	if err != nil {
		return nil, err
	}
	chanConfigs, err := s.tx.Repositories().Game.ListByGameChannel(ctx, pkg.GameChannel)
	if err != nil {
		return nil, err
	}
	chanCfgByPlugin := map[int64]GameChannelPluginConfig{}
	for _, cfg := range chanConfigs {
		chanCfgByPlugin[cfg.PluginIDRef] = cfg
	}
	rowByPlugin := map[int64]ChannelPackagePluginOverride{}
	for _, row := range rows {
		rowByPlugin[row.PluginIDRef] = row
	}
	out := make([]dto.PackagePluginItemView, 0, len(metas))
	for _, meta := range metas {
		if !meta.Enabled || !domainplugin.ValidatePluginRegionCompatibility(pkg.MarketCode, meta.Region) {
			continue
		}
		tpl, err := s.tx.Repositories().Features.GetLatestTemplate(ctx, meta.ID)
		if err != nil {
			return nil, err
		}
		row, ok := rowByPlugin[meta.ID]
		if !ok {
			row = ChannelPackagePluginOverride{
				PackageIDRef:         packageID,
				PluginIDRef:          meta.ID,
				InheritChannelConfig: true,
				ConfigStatus:         string(common.ConfigStatusEmpty),
				ConfigJSON:           map[string]any{},
			}
		}
		runtimeEnabled := row.Enabled
		runtimeStatus := common.ConfigStatus(row.ConfigStatus)
		runtimeConfig := row.ConfigJSON
		if row.InheritChannelConfig {
			if chanCfg, has := chanCfgByPlugin[meta.ID]; has {
				runtimeEnabled = chanCfg.Enabled
				runtimeStatus = common.ConfigStatus(chanCfg.ConfigStatus)
				runtimeConfig = chanCfg.ConfigJSON
			} else {
				runtimeEnabled = false
				runtimeStatus = common.ConfigStatusEmpty
				runtimeConfig = map[string]any{}
			}
		}
		flags := domainplugin.ResolveRuntimeFlags(gc.Hidden, true, runtimeEnabled, runtimeStatus)
		out = append(out, dto.PackagePluginItemView{
			ID:                      row.ID,
			PackageID:               packageID,
			PluginID:                meta.PluginID,
			PluginName:              meta.Name,
			Region:                  meta.Region,
			Required:                meta.Required,
			Selectable:              meta.Selectable,
			Locked:                  meta.Locked,
			InheritChannelConfig:    row.InheritChannelConfig,
			Enabled:                 row.Enabled,
			ConfigJSON:              maskSecrets(runtimeConfig, secretFieldsOf(tpl)),
			ConfigStatus:            row.ConfigStatus,
			LastCheckMessage:        row.LastCheckMessage,
			LastCheckAt:             row.LastCheckAt,
			IncludedInRuntimeConfig: flags.IncludedInRuntimeConfig,
			Template:                templateView(tpl),
		})
	}
	return out, nil
}
