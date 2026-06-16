package payment

import (
	"fmt"
	"strconv"
)

func ValidateRouteSet(routes []Route) error {
	seenPriority := map[string]struct{}{}
	seenSelector := map[string]struct{}{}

	for _, route := range routes {
		if !route.Enabled {
			continue
		}

		priorityKey := strconv.FormatInt(route.PayWayIDRef, 10) + ":" + strconv.Itoa(route.Priority)
		if _, ok := seenPriority[priorityKey]; ok {
			return fmt.Errorf("duplicate priority for pay_way %d", route.PayWayIDRef)
		}
		seenPriority[priorityKey] = struct{}{}

		selectorKey := fmt.Sprintf("%d|%s|%s|%s|%s|%s",
			route.PayWayIDRef,
			normalizeIDValue(route.PackageIDRef),
			normalizeIDValue(route.ChannelIDRef),
			normalizeMarketCode(route.MarketCode),
			normalizeStringValue(route.CountryCode),
			normalizeStringValue(route.Currency),
		)
		if _, ok := seenSelector[selectorKey]; ok {
			return fmt.Errorf("duplicate selector for pay_way %d", route.PayWayIDRef)
		}
		seenSelector[selectorKey] = struct{}{}
	}

	return nil
}
