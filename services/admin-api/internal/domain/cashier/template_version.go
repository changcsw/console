package cashier

type VersionStatus string

const (
	StatusDraft     VersionStatus = "draft"
	StatusPublished VersionStatus = "published"
	StatusArchived  VersionStatus = "archived"
)

type TemplateVersion struct {
	TemplateID string        `json:"templateId,omitempty"`
	Version    int           `json:"version"`
	Status     VersionStatus `json:"status"`
}

func (t TemplateVersion) CopyToDraft(nextVersion int) TemplateVersion {
	return TemplateVersion{
		TemplateID: t.TemplateID,
		Version:    nextVersion,
		Status:     StatusDraft,
	}
}
