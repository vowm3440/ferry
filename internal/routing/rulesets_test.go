package routing

import (
	"encoding/json"
	"ferry/internal/spec"
	"strings"
	"testing"
)

func TestRuleSetAndDomainUseOrAndGeoIPTriggersResolve(t *testing.T) {
	r := spec.Rule{ID: "r", Name: "set", Enabled: true, Hosts: []string{"geosite:cn", "example.com", "geoip:cn"}, Network: "tcp", Action: "direct", ResolveBefore: true}
	value, err := Build(spec.Default(), r)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(value)
	if !strings.Contains(string(data), `"rule_set":["geosite-cn","geoip-cn"]`) || !strings.Contains(string(data), `"mode":"or"`) {
		t.Fatalf("ruleset OR missing: %s", data)
	}
	if !NeedsResolve(r) {
		t.Fatal("geoip did not request resolution")
	}
	resolve, err := BuildResolve(spec.Default(), r)
	if err != nil {
		t.Fatal(err)
	}
	data, _ = json.Marshal(resolve)
	if strings.Contains(string(data), "geoip-cn") || !strings.Contains(string(data), "geosite-cn") {
		t.Fatalf("wrong DNS precondition: %s", data)
	}
}
