package host

import (
	"context"
	"testing"

	"ferry/internal/spec"
	"ferry/internal/telemetry"
)

func TestRuleTesterUsesEngineAndReportsResolutionBoundary(t *testing.T) {
	s := spec.Default()
	s.Rules = []spec.Rule{{ID: "browser", Name: "Browser", Enabled: true, Apps: []string{"chrome.exe"}, Hosts: []string{"*.example.com"}, Ports: []string{"443"}, Network: "tcp", Action: "block"}}
	h := New()
	input := telemetry.RuleTestRequest{Config: s, App: `C:\Apps\CHROME.EXE`, Host: "api.example.com", Port: 443, Network: "tcp"}
	result, err := h.TestRule(context.Background(), input)
	if err != nil || result.MatchedRuleID != "browser" || result.Action != "block" {
		t.Fatalf("engine mismatch: %+v %v", result, err)
	}
	input.App = `C:\other.exe`
	result, err = h.TestRule(context.Background(), input)
	if err != nil || result.MatchedRuleID != "builtin-default" {
		t.Fatalf("default mismatch: %+v %v", result, err)
	}
	input.Host = "127.0.0.1"
	result, err = h.TestRule(context.Background(), input)
	if err != nil || result.MatchedRuleID != "builtin-lan" {
		t.Fatalf("LAN mismatch: %+v %v", result, err)
	}
	input.Host = "api.example.com"
	input.Config.Rules[0].Apps = []string{}
	input.Config.Rules[0].Hosts = []string{"203.0.113.0/24"}
	input.Config.Rules[0].ResolveBefore = true
	result, err = h.TestRule(context.Background(), input)
	if err != nil || result.Stage != "dns-required" || result.Action != "" {
		t.Fatalf("unresolved DNS represented as definite result: %+v %v", result, err)
	}
	input.Port = 0
	if _, err = h.TestRule(context.Background(), input); err == nil {
		t.Fatal("invalid port accepted")
	}
}

func TestRuleTesterDoesNotWidenNetworkOnlyRejectRule(t *testing.T) {
	s := spec.Default()
	s.Rules = []spec.Rule{{ID: "tcp", Name: "TCP only", Enabled: true, Network: "tcp", Action: "block"}}
	result, err := New().TestRule(context.Background(), telemetry.RuleTestRequest{Config: s, Host: "example.com", Port: 443, Network: "udp"})
	if err != nil || result.MatchedRuleID != "builtin-default" {
		t.Fatalf("TCP-only rule matched UDP: %+v %v", result, err)
	}
}
