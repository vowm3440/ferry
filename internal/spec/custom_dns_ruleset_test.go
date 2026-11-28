package spec

import (
	"encoding/json"
	"slices"
	"testing"
)

func TestCustomRuleSetValidationAndReferences(t *testing.T) {
	s := Default()
	s.RuleSets = []RuleSet{
		{ID: "openai", Name: "OpenAI", URL: "https://cdn.example.com/geosite-openai.srs", AutoUpdate: true, UpdateHours: 12},
		{ID: "apple-cn", Name: "Apple CN", URL: "https://raw.githubusercontent.com/MetaCubeX/meta-rules-dat/sing/geo/geosite/apple-cn.srs"},
	}
	s.Rules = append(DefaultRules()[:1],
		Rule{ID: "r1", Name: "AI", Enabled: true, Apps: []string{}, Hosts: []string{"ruleset:openai"}, Ports: []string{}, Network: "any", Action: "proxy", TargetID: "proxy"},
		DefaultRules()[1],
	)
	s.DNS.Rules = []DNSRule{
		{ID: "d1", Name: "Apple DNS", Enabled: true, Domains: []string{"ruleset:apple-cn", "*.corp"}, Server: "direct"},
	}
	if err := s.Validate(); err != nil {
		t.Fatalf("valid custom rule-sets rejected: %v", err)
	}
	refs := s.ReferencedRuleSets()
	if !slices.Contains(refs, "openai") || !slices.Contains(refs, "apple-cn") {
		t.Fatalf("ReferencedRuleSets missed custom sets: %v", refs)
	}
	raw, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Decode(raw); err != nil {
		t.Fatalf("Decode rejected serialized spec with custom ruleSets/DNS: %v", err)
	}
}

func TestCustomRuleSetRejectsUnsafeInputs(t *testing.T) {
	cases := map[string]RuleSet{
		"http":       {ID: "a", Name: "A", URL: "http://example.com/a.srs"},
		"not-srs":    {ID: "a", Name: "A", URL: "https://example.com/a.json"},
		"userinfo":   {ID: "a", Name: "A", URL: "https://user:pass@example.com/a.srs"},
		"builtin-id": {ID: "geosite-cn", Name: "CN", URL: "https://example.com/cn.srs"},
		"bad-id":     {ID: "../escape", Name: "Bad", URL: "https://example.com/a.srs"},
		"bad-hours":  {ID: "a", Name: "A", URL: "https://example.com/a.srs", UpdateHours: 999},
	}
	for name, rs := range cases {
		t.Run(name, func(t *testing.T) {
			s := Default()
			s.RuleSets = []RuleSet{rs}
			if err := s.Validate(); err == nil {
				t.Fatalf("expected %s to fail validation", name)
			}
		})
	}
}

func TestSplitDNSValidation(t *testing.T) {
	s := Default()
	s.DNS.Servers = []DNSServer{
		{ID: "corp-dns", Name: "Corp DNS", Address: "udp://10.0.0.53", Detour: "direct"},
	}
	s.DNS.Rules = []DNSRule{
		{ID: "r-corp", Name: "Corp", Enabled: true, Domains: []string{"*.corp", "*.local", "intranet.example.com"}, Server: "corp-dns"},
		{ID: "r-cn", Name: "CN", Enabled: true, Domains: []string{"geosite:cn"}, Server: "direct"},
	}
	if err := s.Validate(); err != nil {
		t.Fatalf("valid split DNS rejected: %v", err)
	}
	bad := map[string]func(*RunSpec){
		"unknown-server": func(s *RunSpec) { s.DNS.Rules[0].Server = "missing" },
		"geoip-in-dns":   func(s *RunSpec) { s.DNS.Rules[0].Domains = []string{"geoip:cn"} },
		"empty-domains":  func(s *RunSpec) { s.DNS.Rules[0].Domains = nil },
		"bad-detour":     func(s *RunSpec) { s.DNS.Servers[0].Detour = "block" },
		"reserved-id":    func(s *RunSpec) { s.DNS.Servers[0].ID = "direct" },
		"domain-server":  func(s *RunSpec) { s.DNS.Servers[0].Address = "https://dns.example.com/dns-query" },
	}
	for name, mutate := range bad {
		t.Run(name, func(t *testing.T) {
			copySpec := s
			copySpec.DNS.Servers = slices.Clone(s.DNS.Servers)
			copySpec.DNS.Rules = slices.Clone(s.DNS.Rules)
			mutate(&copySpec)
			if err := copySpec.Validate(); err == nil {
				t.Fatalf("expected %s to fail", name)
			}
		})
	}
}
