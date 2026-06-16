package cashier

import "testing"

func TestCopyPublishedVersionCreatesDraft(t *testing.T) {
	source := TemplateVersion{Version: 7, Status: StatusPublished}
	copied := source.CopyToDraft(8)

	if copied.Status != StatusDraft {
		t.Fatalf("expected draft status, got %s", copied.Status)
	}

	if copied.Version != 8 {
		t.Fatalf("expected version 8, got %d", copied.Version)
	}
}
