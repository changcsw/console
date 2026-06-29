package command

import (
	"testing"

	domaincashier "github.com/csw/console/services/admin-api/internal/domain/cashier"
)

// BuildDraftFromTemplateVersion：产物恒 draft，nextVersion 生效，source_type 按来源状态映射。
func TestBuildDraftFromTemplateVersion(t *testing.T) {
	cases := []struct {
		name           string
		sourceStatus   domaincashier.VersionStatus
		wantSourceType domaincashier.SourceType
	}{
		{"from_published", domaincashier.StatusPublished, domaincashier.SourceTypeCopyPublished},
		{"from_archived", domaincashier.StatusArchived, domaincashier.SourceTypeCopyArchived},
		{"from_draft_defaults_manual", domaincashier.StatusDraft, domaincashier.SourceTypeManual},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			source := domaincashier.TemplateVersion{
				TemplateID: "global_default",
				Version:    5,
				Status:     c.sourceStatus,
			}
			draft := BuildDraftFromTemplateVersion(source, 6)

			if draft.Status != domaincashier.StatusDraft {
				t.Fatalf("draft status must be draft, got %s", draft.Status)
			}
			if draft.Version != 6 {
				t.Fatalf("draft version must be nextVersion 6, got %d", draft.Version)
			}
			if draft.TemplateID != "global_default" {
				t.Fatalf("templateId must be preserved, got %q", draft.TemplateID)
			}
			if draft.SourceType != c.wantSourceType {
				t.Fatalf("sourceType=%s want %s", draft.SourceType, c.wantSourceType)
			}
		})
	}
}
