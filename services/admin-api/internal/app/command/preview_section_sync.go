package command

import (
	"github.com/csw/console/services/admin-api/internal/domain/channel"
	domainsync "github.com/csw/console/services/admin-api/internal/domain/sync"
)

type PreviewSectionSyncCommand struct {
	GameID           string
	SelectedSections []string
	IncludeDeletes   bool
}

type SectionPreviewInput struct {
	Sections []string
	Channels []channel.GameMarketChannel
}

type DiffItem struct {
	Key string
}

func NormalizePreviewSectionSync(cmd PreviewSectionSyncCommand) (PreviewSectionSyncCommand, error) {
	sections, err := domainsync.ParseSections(cmd.SelectedSections, false)
	if err != nil {
		return PreviewSectionSyncCommand{}, err
	}

	cmd.SelectedSections = sectionsToStrings(sections)
	return cmd, nil
}

func BuildSectionPreview(input SectionPreviewInput) map[string][]DiffItem {
	result := make(map[string][]DiffItem)

	sections, err := domainsync.ParseSections(input.Sections, false)
	if err != nil {
		return result
	}

	for _, section := range sections {
		if section != domainsync.SectionChannels {
			result[string(section)] = []DiffItem{}
			continue
		}

		diffItems := make([]DiffItem, 0, len(input.Channels))
		for _, item := range input.Channels {
			if item.Hidden {
				continue
			}

			diffItems = append(diffItems, DiffItem{Key: item.ChannelID})
		}

		result[string(section)] = diffItems
	}

	return result
}

func sectionsToStrings(sections []domainsync.Section) []string {
	result := make([]string, 0, len(sections))
	for _, section := range sections {
		result = append(result, string(section))
	}

	return result
}
