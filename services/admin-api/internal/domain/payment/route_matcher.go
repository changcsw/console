package payment

import (
	"sort"
)

func MatchedRoutes(routes []Route, input MatchInput) []Route {
	candidates := make([]Route, 0, len(routes))

	inputPayWay := normalizeSelector(input.PayWay)
	inputPackage := normalizeSelector(input.Package)
	inputChannel := normalizeSelector(input.Channel)
	inputMarket := normalizeUpperSelector(input.Market)
	inputCountry := normalizeUpperSelector(input.Country)
	inputCurrency := normalizeUpperSelector(input.Currency)

	for _, route := range routes {
		if !route.Enabled {
			continue
		}
		if normalizeSelector(route.PayWay) != inputPayWay {
			continue
		}
		if !matchesSelector(route.Package, inputPackage) {
			continue
		}
		if !matchesSelector(route.Channel, inputChannel) {
			continue
		}
		if !marketMatches(route.Market, inputMarket) {
			continue
		}
		if !matchesUpperSelector(route.Country, inputCountry) {
			continue
		}
		if !matchesUpperSelector(route.Currency, inputCurrency) {
			continue
		}
		candidates = append(candidates, route)
	}

	return candidates
}

func PickBestRoute(routes []Route, input MatchInput) (Route, error) {
	candidates := MatchedRoutes(routes, input)
	if len(candidates) == 0 {
		return Route{}, ErrRouteNotFound
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		return compareRouteSpecificity(candidates[i], candidates[j], input) < 0
	})

	return candidates[0], nil
}

func compareRouteSpecificity(left, right Route, input MatchInput) int {
	inputPackage := normalizeSelector(input.Package)
	inputMarket := normalizeUpperSelector(input.Market)
	if rank := packageRank(left.Package, inputPackage) - packageRank(right.Package, inputPackage); rank != 0 {
		return rank
	}

	if rank := marketRank(left.Market, inputMarket) - marketRank(right.Market, inputMarket); rank != 0 {
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

func matchesSelector(routeValue, inputValue string) bool {
	return normalizeSelector(routeValue) == "*" || normalizeSelector(routeValue) == normalizeSelector(inputValue)
}

func matchesUpperSelector(routeValue, inputValue string) bool {
	return normalizeUpperSelector(routeValue) == "*" || normalizeUpperSelector(routeValue) == normalizeUpperSelector(inputValue)
}

func marketMatches(routeMarket, inputMarket string) bool {
	rm := normalizeUpperSelector(routeMarket)
	im := normalizeUpperSelector(inputMarket)
	if rm == "*" {
		return true
	}
	switch im {
	case "CN":
		return rm == "CN"
	case "JP", "KR", "SEA", "HMT":
		return rm == im || rm == "GLOBAL"
	case "GLOBAL":
		return rm == "GLOBAL"
	default:
		return rm == im
	}
}

func packageRank(routePackage, inputPackage string) int {
	if normalizeSelector(routePackage) == "*" {
		return 1
	}
	if normalizeSelector(routePackage) == normalizeSelector(inputPackage) {
		return 0
	}
	return 2
}

func marketRank(routeMarket, inputMarket string) int {
	routeCode := normalizeUpperSelector(routeMarket)
	targetCode := normalizeUpperSelector(inputMarket)

	switch {
	case routeCode == targetCode && routeCode != "GLOBAL" && routeCode != "*":
		return 0
	case routeCode == "GLOBAL":
		return 1
	case routeCode == "*":
		return 2
	default:
		return 3
	}
}

func explicitConditionCount(route Route) int {
	count := 0
	if normalizeSelector(route.Package) != "*" {
		count++
	}
	if normalizeSelector(route.Channel) != "*" {
		count++
	}
	if normalizeUpperSelector(route.Market) != "*" {
		count++
	}
	if normalizeUpperSelector(route.Country) != "*" {
		count++
	}
	if normalizeUpperSelector(route.Currency) != "*" {
		count++
	}

	return count
}
