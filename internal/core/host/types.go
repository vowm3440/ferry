// Package host owns an embedded sing-box instance and its private controller.
package host

type State struct {
	Status      string `json:"status"`
	Mode        string `json:"mode"`
	UptimeMs    int64  `json:"uptimeMs"`
	MemoryBytes int64  `json:"memoryBytes"`
	Version     string `json:"version"`
	CoreVersion string `json:"coreVersion"`
	ServiceMode bool   `json:"serviceMode"`
	Error       string `json:"error,omitempty"`
}

type Traffic struct {
	Up          int64   `json:"up"`
	Down        int64   `json:"down"`
	ActiveConns int64   `json:"activeConns"`
	TotalUp     int64   `json:"totalUp"`
	TotalDown   int64   `json:"totalDown"`
	MemoryBytes int64   `json:"memoryBytes"`
	UpHistory   []int64 `json:"upHistory"`
	DownHistory []int64 `json:"downHistory"`
}
