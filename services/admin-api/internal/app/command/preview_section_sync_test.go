package command

import (
	"testing"

	"github.com/csw/console/services/admin-api/internal/domain/channel"
	"github.com/csw/console/services/admin-api/internal/domain/common"
)

func TestHiddenChannelExcludedFromSyncPreview(t *testing.T) {
	diff := BuildSectionPreview(SectionPreviewInput{
		Sections: []string{"channels"},
		Channels: []channel.GameMarketChannel{
			{
				ChannelID:    "google",
				Hidden:       true,
				ConfigStatus: common.ConfigStatusValid,
			},
		},
	})

	if len(diff["channels"]) != 0 {
		t.Fatal("hidden channel should not appear in preview")
	}
}
