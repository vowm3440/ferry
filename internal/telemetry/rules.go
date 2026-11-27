package telemetry

import "ferry/internal/spec"

type RuleTestRequest struct {
	Config  spec.RunSpec `json:"config"`
	App     string       `json:"app"`
	Host    string       `json:"host"`
	Port    int          `json:"port"`
	Network string       `json:"network"`
}

type RuleTestResult struct {
	MatchedRuleID   string   `json:"matchedRuleId"`
	MatchedRuleName string   `json:"matchedRuleName"`
	Action          string   `json:"action"`
	OutboundTag     string   `json:"outboundTag"`
	Chain           []string `json:"chain"`
	Stage           string   `json:"stage"`
}
