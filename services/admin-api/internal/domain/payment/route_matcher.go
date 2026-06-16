package payment

import (
	"errors"
	"sort"
	"strconv"
	"strings"

	"github.com/csw/console/services/admin-api/internal/domain/common"
)

type RouteMatchInput struct {
	PayWayIDRef  int64
	PackageIDRef *int64
	ChannelIDRef *int64
	MarketCode   string
	CountryCode  string
	Currency     string
}

func PickBestRoute(routes []Route, input RouteMatchInput) (Route, error) {
	candidates := matchedRoutes(routes, input)
	if len(candidates) == 0 {
		return Route{}, errors.New("no matching route")
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		return compareRouteSpecificity(candidates[i], candidates[j], input) < 0
	})

	return candidates[0], nil
}

func matchedRoutes(routes []Route, input RouteMatchInput) []Route {
	candidates := make([]Route, 0, len(routes))

	for _, route := range routes {
		if !route.Enabled || route.PayWayIDRef != input.PayWayIDRef {
			continue
		}

		if !matchIDField(route.PackageIDRef, input.PackageIDRef) {
			continue
		}

		if !matchIDField(route.ChannelIDRef, input.ChannelIDRef) {
			continue
		}

		if !matchMarket(route.MarketCode, input.MarketCode) {
			continue
		}

		if !matchStringField(route.CountryCode, input.CountryCode) {
			continue
		}

		if !matchStringField(route.Currency, input.Currency) {
			continue
		}

		candidates = append(candidates, route)
	}

	return candidates
}

func compareRouteSpecificity(left, right Route, input RouteMatchInput) int {
	if rank := exactIDRank(left.PackageIDRef, input.PackageIDRef) - exactIDRank(right.PackageIDRef, input.PackageIDRef); rank != 0 {
		return rank
	}

	if rank := marketRank(left.MarketCode, input.MarketCode) - marketRank(right.MarketCode, input.MarketCode); rank != 0 {
		return rank
	}

	if rank := explicitConditionCount(right) - explicitConditionCount(left); rank != 0 {
		return rank
	}

	if left.Priority != right.Priority {
		return left.Priority - right.Priority
	}

	return 0
}

func matchIDField(routeValue, inputValue *int64) bool {
	if routeValue == nil {
		return true
	}

	if inputValue == nil {
		return false
	}

	return *routeValue == *inputValue
}

func exactIDRank(routeValue, inputValue *int64) int {
	if routeValue != nil && inputValue != nil && *routeValue == *inputValue {
		return 0
	}

	return 1
}

func matchMarket(routeMarket, targetMarket string) bool {
	routeCode := normalizeMarketCode(routeMarket)
	targetCode := normalizeMarketCode(targetMarket)

	if routeCode != "*" && !common.Market(routeCode).IsKnown() {
		return false
	}

	if targetCode != "*" && !common.Market(targetCode).IsKnown() {
		return false
	}

	switch targetCode {
	case string(common.MarketCN):
		return routeCode == string(common.MarketCN) || routeCode == "*"
	case string(common.MarketGlobal):
		return routeCode == string(common.MarketGlobal) || routeCode == "*"
	case string(common.MarketJP), string(common.MarketKR), string(common.MarketSEA), string(common.MarketHMT):
		return routeCode == targetCode || routeCode == string(common.MarketGlobal) || routeCode == "*"
	default:
		return routeCode == targetCode || routeCode == "*"
	}
}

func marketRank(routeMarket, targetMarket string) int {
	routeCode := normalizeMarketCode(routeMarket)
	targetCode := normalizeMarketCode(targetMarket)

	switch {
	case routeCode == targetCode:
		return 0
	case targetCode != string(common.MarketCN) &&
		targetCode != string(common.MarketGlobal) &&
		routeCode == string(common.MarketGlobal):
		return 1
	case routeCode == "*":
		return 2
	default:
		return 3
	}
}

func matchStringField(routeValue, inputValue string) bool {
	return normalizeStringValue(routeValue) == "*" || normalizeStringValue(routeValue) == normalizeStringValue(inputValue)
}

func explicitConditionCount(route Route) int {
	count := 0
	if route.PackageIDRef != nil {
		count++
	}
	if route.ChannelIDRef != nil {
		count++
	}
	if normalizeMarketCode(route.MarketCode) != "*" {
		count++
	}
	if normalizeStringValue(route.CountryCode) != "*" {
		count++
	}
	if normalizeStringValue(route.Currency) != "*" {
		count++
	}

	return count
}

func normalizeMarketCode(value string) string {
	normalized := strings.ToUpper(strings.TrimSpace(value))
	if normalized == "" || normalized == "*" {
		return "*"
	}

	return normalized
}

func normalizeStringValue(value string) string {
	normalized := strings.ToUpper(strings.TrimSpace(value))
	if normalized == "" || normalized == "*" {
		return "*"
	}

	return normalized
}

func normalizeIDValue(value *int64) string {
	if value == nil {
		return "*"
	}

	return strconv.FormatInt(*value, 10)
}
