package command

import "testing"

func TestNormalizePreviewSectionSyncDefaultsAllSections(t *testing.T) {
	cmd, err := NormalizePreviewSectionSync(PreviewSectionSyncCommand{GameID: "game-1"})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(cmd.SelectedSections) != 9 {
		t.Fatalf("want 9 sections, got %d", len(cmd.SelectedSections))
	}
}
