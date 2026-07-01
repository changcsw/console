package snapshot

import (
	"slices"
	"strings"

	"github.com/csw/console/services/admin-api/internal/domain/common"
	"github.com/csw/console/services/admin-api/internal/domain/plugin"
)

var allMarkets = []common.Market{
	common.MarketGlobal,
	common.MarketJP,
	common.MarketKR,
	common.MarketSEA,
	common.MarketHMT,
	common.MarketCN,
}

func BuildRuntimeConfig(view ValidDataView) RuntimeConfig {
	runtime := RuntimeConfig{
		SchemaVersion: ConfigSchemaVersion,
		GameID:        view.GameID,
		GeneratedAt:   view.GeneratedAt.UTC().Format("2006-01-02T15:04:05Z"),
		Markets:       make(map[string]MarketConfig, len(allMarkets)),
	}

	baseGame := buildGameBaseConfig(view)
	channelByMarket := buildChannelsByMarket(view.Channels)
	for _, market := range allMarkets {
		marketChannels := resolveMarketChannels(market, channelByMarket)
		routes := append([]ResolvedRoute(nil), view.PaymentRoutes[market]...)
		slices.SortFunc(routes, func(a, b ResolvedRoute) int {
			return strings.Compare(a.PayWay, b.PayWay)
		})
		runtime.Markets[string(market)] = MarketConfig{
			Game:          baseGame,
			Channels:      marketChannels,
			PaymentRoutes: routes,
		}
	}
	return runtime
}

func buildGameBaseConfig(view ValidDataView) GameBaseConfig {
	accountAuth := make([]map[string]any, 0, len(view.AccountAuth))
	for _, item := range view.AccountAuth {
		filtered := filterTemplateConfig(item.Config, item.FormSchema, item.SecretFields)
		if len(filtered) == 0 {
			continue
		}
		filtered["authTypeId"] = item.AuthTypeID
		accountAuth = append(accountAuth, filtered)
	}
	slices.SortFunc(accountAuth, func(a, b map[string]any) int {
		return strings.Compare(asString(a["authTypeId"]), asString(b["authTypeId"]))
	})

	game := GameBaseConfig{
		LegalLinks:  append([]LegalLink(nil), view.LegalLinks...),
		AccountAuth: accountAuth,
		Products:    append([]ProductItem(nil), view.Products...),
	}
	if view.Cashier != nil {
		game.Cashier = map[string]any{
			"templateId":       view.Cashier.TemplateID,
			"templateVersion":  view.Cashier.TemplateVersion,
			"snapshotChecksum": view.Cashier.SnapshotChecksum,
		}
	}
	return game
}

func buildChannelsByMarket(channels []ChannelInput) map[common.Market][]ResolvedChannel {
	out := make(map[common.Market][]ResolvedChannel)
	for _, channel := range channels {
		if !isChannelValid(channel) {
			continue
		}
		resolved := ResolvedChannel{
			ChannelID: channel.ChannelID,
			Region:    channel.Region,
			Login:     resolveTemplatePart(channel.Login),
			IAP:       resolveTemplatePart(channel.IAP),
			Plugins:   resolvePlugins(channel),
			Packages:  filterEnabledPackages(channel.Packages),
		}
		out[channel.Market] = append(out[channel.Market], resolved)
	}
	for market := range out {
		slices.SortFunc(out[market], func(a, b ResolvedChannel) int {
			return strings.Compare(a.ChannelID, b.ChannelID)
		})
	}
	return out
}

func resolveMarketChannels(target common.Market, channelByMarket map[common.Market][]ResolvedChannel) []ResolvedChannel {
	switch target {
	case common.MarketCN:
		channels := append([]ResolvedChannel(nil), channelByMarket[common.MarketCN]...)
		attachSourceMarket(channels, common.MarketCN)
		return channels
	case common.MarketGlobal:
		channels := append([]ResolvedChannel(nil), channelByMarket[common.MarketGlobal]...)
		attachSourceMarket(channels, common.MarketGlobal)
		return channels
	default:
		globalChannels := append([]ResolvedChannel(nil), channelByMarket[common.MarketGlobal]...)
		marketChannels := append([]ResolvedChannel(nil), channelByMarket[target]...)
		return mergeByInstance(globalChannels, marketChannels, target)
	}
}

func mergeByInstance(globalChannels []ResolvedChannel, marketChannels []ResolvedChannel, market common.Market) []ResolvedChannel {
	merged := make(map[string]ResolvedChannel, len(globalChannels)+len(marketChannels))
	for _, item := range globalChannels {
		item.SourceMarket = string(common.MarketGlobal)
		merged[item.ChannelID] = item
	}
	for _, item := range marketChannels {
		item.SourceMarket = string(market)
		merged[item.ChannelID] = item
	}

	out := make([]ResolvedChannel, 0, len(merged))
	for _, item := range merged {
		out = append(out, item)
	}
	slices.SortFunc(out, func(a, b ResolvedChannel) int {
		return strings.Compare(a.ChannelID, b.ChannelID)
	})
	return out
}

func attachSourceMarket(channels []ResolvedChannel, market common.Market) {
	for i := range channels {
		channels[i].SourceMarket = string(market)
	}
}

func isChannelValid(channel ChannelInput) bool {
	if channel.Hidden || !channel.Enabled || channel.ConfigStatus != common.ConfigStatusValid {
		return false
	}
	if !isMarketRegionCompatible(channel.Market, channel.Region) {
		return false
	}
	if channel.Login != nil && (channel.Login.ConfigStatus != common.ConfigStatusValid || !channel.Login.Enabled) {
		return false
	}
	if channel.IAP != nil && (channel.IAP.ConfigStatus != common.ConfigStatusValid || !channel.IAP.Enabled) {
		return false
	}
	for _, p := range channel.Plugins {
		flags := plugin.ResolveRuntimeFlags(
			channel.Hidden,
			isMarketRegionCompatible(channel.Market, p.Region),
			p.Enabled,
			p.ConfigStatus,
		)
		if p.Required && !flags.IncludedInRuntimeConfig {
			return false
		}
	}
	return true
}

func resolveTemplatePart(in *TemplateConfig) map[string]any {
	if in == nil || !in.Enabled || in.ConfigStatus != common.ConfigStatusValid {
		return nil
	}
	return filterTemplateConfig(in.Config, in.FormSchema, in.SecretFields)
}

func resolvePlugins(channel ChannelInput) []map[string]any {
	items := make([]map[string]any, 0)
	for _, p := range channel.Plugins {
		flags := plugin.ResolveRuntimeFlags(
			channel.Hidden,
			isMarketRegionCompatible(channel.Market, p.Region),
			p.Enabled,
			p.ConfigStatus,
		)
		if !flags.IncludedInRuntimeConfig {
			continue
		}
		cfg := filterTemplateConfig(p.Config, p.FormSchema, p.SecretFields)
		cfg["pluginId"] = p.PluginID
		cfg["pluginName"] = p.PluginName
		cfg["region"] = p.Region
		items = append(items, cfg)
	}
	slices.SortFunc(items, func(a, b map[string]any) int {
		return strings.Compare(asString(a["pluginId"]), asString(b["pluginId"]))
	})
	return items
}

func filterEnabledPackages(in []PackageConfig) []PackageConfig {
	out := make([]PackageConfig, 0, len(in))
	for _, item := range in {
		if item.Enabled {
			out = append(out, item)
		}
	}
	slices.SortFunc(out, func(a, b PackageConfig) int {
		return strings.Compare(a.PackageCode, b.PackageCode)
	})
	return out
}

func filterTemplateConfig(config map[string]any, fields []ScopeField, secretFields []string) map[string]any {
	if len(config) == 0 {
		return map[string]any{}
	}
	fieldScope := make(map[string]string, len(fields))
	for _, f := range fields {
		scope := strings.ToLower(strings.TrimSpace(f.Scope))
		if scope == "" {
			scope = "both"
		}
		fieldScope[f.Key] = scope
	}
	secret := make(map[string]struct{}, len(secretFields))
	for _, key := range secretFields {
		secret[key] = struct{}{}
	}

	filtered := make(map[string]any)
	for k, v := range config {
		scope := fieldScope[k]
		if scope == "" {
			scope = "both"
		}
		if scope == "server" {
			continue
		}
		if _, ok := secret[k]; ok {
			filtered[k] = SecretMaskedValue
			continue
		}
		filtered[k] = v
	}
	return filtered
}

func isMarketRegionCompatible(market common.Market, region string) bool {
	region = strings.ToLower(strings.TrimSpace(region))
	if market == common.MarketCN {
		return region == "domestic"
	}
	return region == "overseas"
}

func asString(v any) string {
	s, _ := v.(string)
	return s
}
