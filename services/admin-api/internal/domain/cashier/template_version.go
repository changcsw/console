package cashier

type VersionStatus string

const (
	StatusDraft     VersionStatus = "draft"
	StatusPublished VersionStatus = "published"
	StatusArchived  VersionStatus = "archived"
)

type SourceType string

const (
	SourceTypeManual        SourceType = "manual"
	SourceTypeCopyPublished SourceType = "copy_published"
	SourceTypeCopyArchived  SourceType = "copy_archived"
	SourceTypeFXAuto        SourceType = "fx_auto"
)

type TemplateVersion struct {
	TemplateID string        `json:"templateId,omitempty"`
	Version    int           `json:"version"`
	Status     VersionStatus `json:"status"`
	SourceType SourceType    `json:"sourceType,omitempty"`
}

func (t TemplateVersion) CopyToDraft(nextVersion int) TemplateVersion {
	return TemplateVersion{
		TemplateID: t.TemplateID,
		Version:    nextVersion,
		Status:     StatusDraft,
		SourceType: t.SourceType,
	}
}

// CanTransition 校验版本状态机：仅允许 draft->published、published->archived。
func CanTransition(from, to VersionStatus) bool {
	return (from == StatusDraft && to == StatusPublished) ||
		(from == StatusPublished && to == StatusArchived)
}
