package query

import (
	"strings"

	"github.com/csw/console/services/admin-api/internal/app/dto"
)

type ListMarketChannelsQuery struct {
	GameID string
	Market string
}

func FilterMarketChannels(q ListMarketChannelsQuery, items []dto.GameMarketChannelListItem) []dto.GameMarketChannelListItem {
	if strings.TrimSpace(q.Market) == "" {
		return append([]dto.GameMarketChannelListItem(nil), items...)
	}

	filtered := make([]dto.GameMarketChannelListItem, 0, len(items))
	for _, item := range items {
		if item.Market == q.Market {
			filtered = append(filtered, item)
		}
	}

	return filtered
}
