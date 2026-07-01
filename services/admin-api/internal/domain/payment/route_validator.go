package payment

import (
	"strconv"
	"strings"
)

type conflictPos struct {
	index int
	id    int64
}

func ValidateRouteSet(routes []Route) error {
	seenPriority := map[string]conflictPos{}
	seenSelector := map[string]conflictPos{}

	for i, route := range routes {
		if !route.Enabled {
			continue
		}

		payWay := normalizeSelector(route.PayWay)
		priorityKey := payWay + ":" + strconv.Itoa(route.Priority)
		if prev, ok := seenPriority[priorityKey]; ok {
			return &RouteConflictError{
				Kind:       ConflictDuplicatePriority,
				PayWay:     payWay,
				Priority:   route.Priority,
				LeftIndex:  prev.index,
				RightIndex: i,
				LeftID:     prev.id,
				RightID:    route.ID,
			}
		}
		seenPriority[priorityKey] = conflictPos{index: i, id: route.ID}

		selectorKey := strings.Join([]string{
			payWay,
			normalizeSelector(route.Package),
			normalizeSelector(route.Channel),
			normalizeUpperSelector(route.Market),
			normalizeUpperSelector(route.Country),
			normalizeUpperSelector(route.Currency),
		}, "|")
		if prev, ok := seenSelector[selectorKey]; ok {
			return &RouteConflictError{
				Kind:       ConflictDuplicateSelector,
				PayWay:     payWay,
				Selector:   selectorKey,
				LeftIndex:  prev.index,
				RightIndex: i,
				LeftID:     prev.id,
				RightID:    route.ID,
			}
		}
		seenSelector[selectorKey] = conflictPos{index: i, id: route.ID}
	}

	return nil
}
