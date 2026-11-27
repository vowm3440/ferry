package spec

import "testing"

func TestOnlyKnownLogicalRuleSetReferencesAccepted(t *testing.T) {
	s := Default()
	for _, value := range []string{"geosite:cn", "geosite:geolocation-!cn", "geoip:cn", "geoip:private"} {
		r := Rule{ID: "r", Name: "set", Network: "any", Action: "direct", Hosts: []string{value}}
		if err := s.ValidateRule(r); err != nil {
			t.Fatalf("known set rejected: %s: %v", value, err)
		}
	}
	for _, value := range []string{"geosite:../secret", "geoip:C:/secret", "ruleset://arbitrary", "geoip:unknown", "geosite:CN"} {
		r := Rule{ID: "r", Name: "set", Network: "any", Action: "direct", Hosts: []string{value}}
		if err := s.ValidateRule(r); err == nil {
			t.Fatalf("unknown/unsafe set accepted: %s", value)
		}
	}
}
