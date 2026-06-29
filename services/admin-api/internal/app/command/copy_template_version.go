package command

import domaincashier "github.com/csw/console/services/admin-api/internal/domain/cashier"

type CopyTemplateVersionCommand struct {
	TemplateID    string
	SourceVersion int
	SourceStatus  domaincashier.VersionStatus
	RequestedBy   string
}

func BuildDraftFromTemplateVersion(source domaincashier.TemplateVersion, nextVersion int) domaincashier.TemplateVersion {
	draft := source.CopyToDraft(nextVersion)
	switch source.Status {
	case domaincashier.StatusPublished:
		draft.SourceType = domaincashier.SourceTypeCopyPublished
	case domaincashier.StatusArchived:
		draft.SourceType = domaincashier.SourceTypeCopyArchived
	default:
		draft.SourceType = domaincashier.SourceTypeManual
	}
	return draft
}
