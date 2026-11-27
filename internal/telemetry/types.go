// Package telemetry contains credential-free control-plane data contracts.
package telemetry

type Connection struct {
	ID          string   `json:"id"`
	Process     string   `json:"process"`
	ProcessPath string   `json:"processPath"`
	Host        string   `json:"host"`
	Port        int      `json:"port"`
	RuleName    string   `json:"ruleName"`
	RuleID      string   `json:"ruleId,omitempty"`
	Action      string   `json:"action"`
	OutboundTag string   `json:"outboundTag"`
	Chain       []string `json:"chain"`
	Up          int64    `json:"up"`
	Down        int64    `json:"down"`
	StartAt     int64    `json:"startAt"`
}

type LogEntry struct {
	Seq     int64  `json:"seq"`
	Time    int64  `json:"time"`
	Level   string `json:"level"`
	Message string `json:"message"`
}

type LogRequest struct {
	After int64 `json:"after"`
	Limit int   `json:"limit"`
}
