package channels

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/csw/console/services/admin-api/internal/app/command"
	"github.com/csw/console/services/admin-api/internal/app/dto"
	"github.com/csw/console/services/admin-api/internal/app/query"
	domainchannel "github.com/csw/console/services/admin-api/internal/domain/channel"
)

type fakeCreateMarketChannelService struct{}

func (fakeCreateMarketChannelService) Create(_ context.Context, _ command.CreateMarketChannelCommand) (domainchannel.GameMarketChannel, error) {
	return domainchannel.GameMarketChannel{}, nil
}

type fakeListMarketChannelsService struct {
	items []dto.GameMarketChannelListItem
}

func (f fakeListMarketChannelsService) List(_ context.Context, _ query.ListMarketChannelsQuery) ([]dto.GameMarketChannelListItem, error) {
	return f.items, nil
}

type fakeMarketChannelVisibilityService struct{}

func (fakeMarketChannelVisibilityService) Hide(_ context.Context, _ command.HideMarketChannelCommand) (domainchannel.GameMarketChannel, error) {
	return domainchannel.GameMarketChannel{}, nil
}

func (fakeMarketChannelVisibilityService) Unhide(_ context.Context, _ command.UnhideMarketChannelCommand) (domainchannel.GameMarketChannel, error) {
	return domainchannel.GameMarketChannel{}, nil
}

func TestCreateMarketChannelRejectsDomesticChannelForJP(t *testing.T) {
	body := strings.NewReader(`{"channelId":"bilibili","region":"domestic"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/admin/games/game-1/markets/JP/channels", body)
	rec := httptest.NewRecorder()

	handler := NewHandler(
		fakeCreateMarketChannelService{},
		fakeListMarketChannelsService{},
		fakeMarketChannelVisibilityService{},
	)
	handler.CreateMarketChannel(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}

	if !strings.Contains(rec.Body.String(), "only accepts overseas channels") {
		t.Fatalf("expected compatibility error, got %s", rec.Body.String())
	}
}

func TestListMarketChannelsReturnsAllMarketsByDefault(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/admin/games/game-1/market-channels", nil)
	rec := httptest.NewRecorder()

	handler := NewHandler(
		fakeCreateMarketChannelService{},
		fakeListMarketChannelsService{
			items: []dto.GameMarketChannelListItem{
				{Market: "GLOBAL", ChannelID: "google"},
				{Market: "JP", ChannelID: "google"},
			},
		},
		fakeMarketChannelVisibilityService{},
	)
	handler.ListMarketChannels(rec, req)

	body := rec.Body.String()
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	if !strings.Contains(body, `"market":"GLOBAL"`) || !strings.Contains(body, `"market":"JP"`) {
		t.Fatalf("expected GLOBAL and JP rows, got %s", body)
	}
}
