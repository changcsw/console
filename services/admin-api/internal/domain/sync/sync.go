package sync

import (
	"fmt"
	"strings"
)

type Section string

const (
	SectionGame     Section = "game"
	SectionMarkets  Section = "markets"
	SectionLegal    Section = "legal"
	SectionChannels Section = "channels"
	SectionPackages Section = "packages"
	SectionProducts Section = "products"
	SectionCashier  Section = "cashier"
	SectionPayments Section = "payments"
	SectionConfig   Section = "config"
)

var allSections = []Section{
	SectionGame,
	SectionMarkets,
	SectionLegal,
	SectionChannels,
	SectionPackages,
	SectionProducts,
	SectionCashier,
	SectionPayments,
	SectionConfig,
}

type Preview struct {
	GameID           string        `json:"gameId"`
	SourceEnv        string        `json:"sourceEnv"`
	TargetEnv        string        `json:"targetEnv"`
	SourceHash       string        `json:"sourceHash"`
	TargetHashBefore string        `json:"targetHashBefore"`
	HasDiff          bool          `json:"hasDiff"`
	Sections         []DiffSection `json:"sections"`
}

type DiffSection struct {
	Section string       `json:"section"`
	Changes []DiffChange `json:"changes"`
}

type DiffChange struct {
	Op              string `json:"op"`
	EntityType      string `json:"entityType"`
	EntityKey       string `json:"entityKey"`
	FieldName       string `json:"fieldName"`
	SandboxValue    any    `json:"sandboxValue,omitempty"`
	ProductionValue any    `json:"productionValue,omitempty"`
	Masked          bool   `json:"masked"`
}

func (s Section) IsKnown() bool {
	switch s {
	case SectionGame, SectionMarkets, SectionLegal, SectionChannels, SectionPackages, SectionProducts, SectionCashier, SectionPayments, SectionConfig:
		return true
	default:
		return false
	}
}

func AllSections() []Section {
	result := make([]Section, len(allSections))
	copy(result, allSections)
	return result
}

func ParseSections(values []string, requireExplicit bool) ([]Section, error) {
	if len(values) == 0 {
		if requireExplicit {
			return nil, fmt.Errorf("selected_sections is required")
		}

		return AllSections(), nil
	}

	seen := make(map[Section]struct{}, len(values))
	result := make([]Section, 0, len(values))
	for _, raw := range values {
		section := Section(strings.TrimSpace(raw))
		if !section.IsKnown() {
			return nil, fmt.Errorf("unknown section: %s", raw)
		}

		if _, exists := seen[section]; exists {
			continue
		}

		seen[section] = struct{}{}
		result = append(result, section)
	}

	return result, nil
}
