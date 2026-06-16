package sync

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

