package telemetry

type RuleSetInfo struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	URL       string `json:"url"`
	UpdatedAt int64  `json:"updatedAt"`
	SizeBytes int64  `json:"sizeBytes"`
	Status    string `json:"status"`
}
