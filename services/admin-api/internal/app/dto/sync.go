package dto

type SyncPreviewRequest struct {
	IncludeDeletes   bool     `json:"includeDeletes"`
	SelectedSections []string `json:"selectedSections"`
}

type SyncExecuteRequest struct {
	IncludeDeletes   bool     `json:"includeDeletes"`
	OperatorNote     string   `json:"operatorNote"`
	SelectedSections []string `json:"selectedSections"`
	BaselineToken    string   `json:"baselineToken"`
}
