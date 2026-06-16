package dto

type CreateGameRequest struct {
	Name              string   `json:"name"`
	Alias             string   `json:"alias"`
	DefaultMarketCode string   `json:"defaultMarketCode"`
	IconURL           string   `json:"iconUrl"`
	Markets           []string `json:"markets"`
}

type UpdateGameRequest struct {
	Name              *string `json:"name,omitempty"`
	Alias             *string `json:"alias,omitempty"`
	DefaultMarketCode *string `json:"defaultMarketCode,omitempty"`
	IconURL           *string `json:"iconUrl,omitempty"`
	Status            *string `json:"status,omitempty"`
}

