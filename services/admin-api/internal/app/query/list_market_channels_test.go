package query

import (
	"testing"

	"github.com/csw/console/services/admin-api/internal/app/dto"
)

func TestListMarketChannelsDefaultsToAllMarkets(t *testing.T) {
	queryInput := ListMarketChannelsQuery{GameID: "game-1"}
	items := []dto.GameMarketChannelListItem{
		{Market: "GLOBAL", ChannelID: "google"},
		{Market: "JP", ChannelID: "google"},
	}

	got := FilterMarketChannels(queryInput, items)
	if len(got) != 2 {
		t.Fatalf("expected 2 items, got %d", len(got))
	}
}
