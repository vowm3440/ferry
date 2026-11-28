package routing

import (
	"encoding/json"
	"strings"
	"testing"

	"ferry/internal/spec"
)

func TestBuildCustomRuleSetAndDNSDomains(t *testing.T) {
	s := spec.Default()
	s.RuleSets = []spec.RuleSet{{ID: "openai", Name: "OpenAI", URL: "https://example.com/openai.srs"}}
	r := spec.Rule{ID: "r1", Name: "AI", Enabled: true, Hosts: []string{"ruleset:openai", "*.openai.com"}, Network: "any", Action: "proxy", TargetID: "proxy"}
	built, err := Build(s, r)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(built)
	if !strings.Contains(string(data), `"rule_set":["openai"]`) || !strings.Contains(string(data), `"domain_suffix":[".openai.com"]`) {
		t.Fatalf("unexpected compiled rule: %s", data)
	}
	dnsCond := BuildDNSDomains(s, []string{"*.corp", "geosite:cn", "ruleset:openai"})
	dnsJSON, _ := json.Marshal(dnsCond)
	if !strings.Contains(string(dnsJSON), `"rule_set":["geosite-cn","openai"]`) || !strings.Contains(string(dnsJSON), `".corp"`) {
		t.Fatalf("unexpected DNS matcher: %s", dnsJSON)
	}
}

func TestBuildLazyPresetGeneratesValidCompleteConfig(t *testing.T) {
	base := spec.Default()
	opts := DefaultLazyPresetOptions()
	opts.IntranetDNS = "udp://10.0.0.53"
	next, err := BuildLazyPreset(base, opts)
	if err != nil {
		t.Fatalf("BuildLazyPreset failed: %v", err)
	}
	if len(next.RuleSets) < 5 || len(next.AppGroups) < 3 || len(next.Rules) < 10 {
		t.Fatalf("incomplete preset: sets=%d groups=%d rules=%d", len(next.RuleSets), len(next.AppGroups), len(next.Rules))
	}
	if len(next.DNS.Servers) != 1 || next.DNS.Servers[0].ID != "corp-intranet" || len(next.DNS.Rules) != 2 {
		t.Fatalf("unexpected split DNS: %+v", next.DNS)
	}
	if next.Rules[0].ID != spec.BuiltinLANRuleID || next.Rules[len(next.Rules)-1].ID != spec.BuiltinDefaultRuleID {
		t.Fatal("preset violated pinned rule order")
	}
}

func TestBuildLazyPresetMergePreservesUserRules(t *testing.T) {
	base := spec.Default()
	base.Rules = append(spec.DefaultRules()[:1],
		spec.Rule{ID: "user-custom", Name: "My Rule", Enabled: true, Hosts: []string{"my.example.com"}, Network: "any", Action: "direct"},
		spec.DefaultRules()[1],
	)
	opts := DefaultLazyPresetOptions()
	opts.ReplaceExisting = false
	next, err := BuildLazyPreset(base, opts)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, r := range next.Rules {
		if r.ID == "user-custom" {
			found = true
		}
	}
	if !found {
		t.Fatal("merge mode dropped existing user rule")
	}
}
