package command

import domainsync "github.com/csw/console/services/admin-api/internal/domain/sync"

type ExecuteSectionSyncCommand struct {
	GameID           string
	SelectedSections []string
	IncludeDeletes   bool
	OperatorNote     string
}

func NormalizeExecuteSectionSync(cmd ExecuteSectionSyncCommand) (ExecuteSectionSyncCommand, error) {
	sections, err := domainsync.ParseSections(cmd.SelectedSections, true)
	if err != nil {
		return ExecuteSectionSyncCommand{}, err
	}

	cmd.SelectedSections = sectionsToStrings(sections)
	return cmd, nil
}
