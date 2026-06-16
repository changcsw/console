package command

import domaincashier "github.com/csw/console/services/admin-api/internal/domain/cashier"

type CopyTemplateVersionCommand struct {
	TemplateID    string
	SourceVersion int
	SourceStatus  domaincashier.VersionStatus
	RequestedBy   string
}

func BuildDraftFromTemplateVersion(source domaincashier.TemplateVersion, nextVersion int) domaincashier.TemplateVersion {
	return source.CopyToDraft(nextVersion)
}
