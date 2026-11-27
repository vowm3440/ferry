package spec

import (
	"encoding/json"
	"strings"
	"testing"
)

func userRule() Rule {
	return Rule{ID: "r", Name: "用户规则", Enabled: true, Network: "any", Action: "proxy", TargetID: "n1", ResolveBefore: true}
}

func TestValidateAllowsSupportedUserRules(t *testing.T) {
	s := sample()
	r := userRule()
	r.Apps = []string{"chrome.exe", `C:\Program Files\App (1)\*.exe`}
	r.Hosts = []string{"EXAMPLE.com", "*.example.com", "cdn?.test", "10.0.0.0/8", "2001:db8::1-2001:db8::ff"}
	r.Ports = []string{"1", "443", "8000-9000", "65535"}
	s.Rules = []Rule{r}
	if err := s.Validate(); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Decode(data); err != nil {
		t.Fatal(err)
	}
}

func TestRuleValidationRejectsUnsupportedAndMalformedValues(t *testing.T) {
	for name, mutate := range map[string]func(*Rule){
		"empty id": func(r *Rule) { r.ID = "" }, "empty name": func(r *Rule) { r.Name = "" },
		"action": func(r *Rule) { r.Action = "reject" }, "network": func(r *Rule) { r.Network = "icmp" },
		"target": func(r *Rule) { r.TargetID = "missing" }, "direct target": func(r *Rule) { r.Action = "direct" },
		"app group":          func(r *Rule) { r.Apps = []string{"@group:games"} },
		"blank app":          func(r *Rule) { r.Apps = []string{" "} },
		"unknown app syntax": func(r *Rule) { r.Apps = []string{"regex:chrome.*"} },
		"drive relative app": func(r *Rule) { r.Apps = []string{`C:chrome.exe`} },
		"geoip":              func(r *Rule) { r.Hosts = []string{"geoip:unknown"} },
		"geosite":            func(r *Rule) { r.Hosts = []string{"geosite:unknown"} },
		"unknown syntax":     func(r *Rule) { r.Hosts = []string{"regexp:SECRET"} },
		"host URL":           func(r *Rule) { r.Hosts = []string{"https://SECRET"} },
		"bad domain":         func(r *Rule) { r.Hosts = []string{"a..example.com"} },
		"CIDR":               func(r *Rule) { r.Hosts = []string{"10.0.0.0/33"} },
		"IPv4":               func(r *Rule) { r.Hosts = []string{"999.0.0.1"} },
		"invalid IP range":   func(r *Rule) { r.Hosts = []string{"999.0.0.0-999.0.0.9"} },
		"reverse IP":         func(r *Rule) { r.Hosts = []string{"10.0.0.8-10.0.0.1"} },
		"mixed family":       func(r *Rule) { r.Hosts = []string{"10.0.0.1-::1"} },
		"zero port":          func(r *Rule) { r.Ports = []string{"0"} },
		"overflow port":      func(r *Rule) { r.Ports = []string{"65536"} },
		"reverse ports":      func(r *Rule) { r.Ports = []string{"90-80"} },
		"port syntax":        func(r *Rule) { r.Ports = []string{"80:90"} },
		"pinned user":        func(r *Rule) { r.Pinned = true },
	} {
		t.Run(name, func(t *testing.T) {
			s, r := sample(), userRule()
			mutate(&r)
			s.Rules = []Rule{r}
			if err := s.Validate(); err == nil || strings.Contains(err.Error(), "SECRET") {
				t.Fatalf("invalid rule accepted or error leaks value: %v", err)
			}
		})
	}
}

func TestDefaultRulesCannotBeAlteredOrReordered(t *testing.T) {
	for _, mutate := range []func(*RunSpec){
		func(s *RunSpec) { s.Rules[0].Enabled = false }, func(s *RunSpec) { s.Rules[0].Hosts = nil },
		func(s *RunSpec) { s.Rules[1].Action = "direct" }, func(s *RunSpec) { s.Rules[1].Pinned = false },
		func(s *RunSpec) { s.Rules[0], s.Rules[1] = s.Rules[1], s.Rules[0] },
		func(s *RunSpec) { s.Rules = append(s.Rules, s.Rules[1]) },
	} {
		s := sample()
		s.Rules = DefaultRules()
		mutate(&s)
		if err := s.Validate(); err == nil {
			t.Fatal("modified or reordered defaults accepted")
		}
	}
	for _, rules := range [][]Rule{nil, {}, DefaultRules()} {
		s := sample()
		s.Rules = rules
		if err := s.Validate(); err != nil {
			t.Fatalf("compatible default configuration rejected: %v", err)
		}
	}
}

func TestRuleBoundsDuplicatesAndUnknownJSONFields(t *testing.T) {
	s := sample()
	s.Rules = []Rule{userRule(), userRule()}
	if err := s.Validate(); err == nil {
		t.Fatal("duplicate rule ID accepted")
	}
	s.Rules = make([]Rule, MaxRules+1)
	if err := s.Validate(); err == nil {
		t.Fatal("unbounded rules accepted")
	}
	r := userRule()
	for _, field := range []*[]string{&r.Apps, &r.Hosts, &r.Ports} {
		*field = make([]string, MaxRuleItems+1)
		s.Rules = []Rule{r}
		if err := s.Validate(); err == nil {
			t.Fatal("unbounded rule conditions accepted")
		}
		*field = nil
	}
	for _, payload := range []string{
		`{"rules":[{"id":"r","name":"R","enabled":true,"network":"any","action":"block","unknown":1}]}`,
		`{"rules":[{"id":"r","name":"R","enabled":true,"Network":"any","action":"block"}]}`,
	} {
		if _, err := Decode([]byte(payload)); err == nil {
			t.Fatal("unknown rule field accepted")
		}
	}
}
