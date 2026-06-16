package dto

type SyncPreviewRequest struct {
	IncludeDeletes bool `json:"includeDeletes"`
}

type SyncExecuteRequest struct {
	IncludeDeletes bool   `json:"includeDeletes"`
	OperatorNote   string `json:"operatorNote"`
}

