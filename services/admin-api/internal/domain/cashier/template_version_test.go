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

// CopyToDraft 产物状态恒 draft，且保留 templateId 与来源 sourceType（00 §3.3 copy-to-draft）。
func TestCopyToDraftPreservesIdentityAndSourceType(t *testing.T) {
	source := TemplateVersion{
		TemplateID: "global_default",
		Version:    3,
		Status:     StatusArchived,
		SourceType: SourceTypeCopyPublished,
	}
	copied := source.CopyToDraft(9)

	if copied.Status != StatusDraft {
		t.Fatalf("status must always be draft, got %s", copied.Status)
	}
	if copied.TemplateID != "global_default" {
		t.Fatalf("templateId must be preserved, got %q", copied.TemplateID)
	}
	if copied.Version != 9 {
		t.Fatalf("version must be nextVersion, got %d", copied.Version)
	}
	if copied.SourceType != SourceTypeCopyPublished {
		t.Fatalf("sourceType must be preserved by CopyToDraft, got %s", copied.SourceType)
	}
}

// CanTransition 仅允许 draft→published、published→archived，其余一律拒绝（含同态/反向/跳态）。
func TestCanTransitionMatrix(t *testing.T) {
	cases := []struct {
		from VersionStatus
		to   VersionStatus
		want bool
	}{
		{StatusDraft, StatusPublished, true},
		{StatusPublished, StatusArchived, true},

		// 非法流转：
		{StatusDraft, StatusArchived, false},     // 跳过 published
		{StatusArchived, StatusPublished, false}, // 需 copy-to-draft
		{StatusArchived, StatusDraft, false},
		{StatusPublished, StatusDraft, false},
		{StatusDraft, StatusDraft, false},
		{StatusPublished, StatusPublished, false},
		{StatusArchived, StatusArchived, false},
		{StatusDraft, "bogus", false},
		{"bogus", StatusPublished, false},
	}
	for _, c := range cases {
		if got := CanTransition(c.from, c.to); got != c.want {
			t.Errorf("CanTransition(%s,%s)=%v want %v", c.from, c.to, got, c.want)
		}
	}
}
