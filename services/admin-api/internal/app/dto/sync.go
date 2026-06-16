package dto

type SyncPreviewRequest struct {
	IncludeDeletes   bool     `json:"includeDeletes"`
	SelectedSections []string `json:"selected_sections"`
}

type SyncExecuteRequest struct {
	IncludeDeletes   bool     `json:"includeDeletes"`
	OperatorNote     string   `json:"operatorNote"`
	SelectedSections []string `json:"selected_sections"`
}
