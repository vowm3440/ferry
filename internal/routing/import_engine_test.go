package routing

import (
	"encoding/json"
	"testing"
)

// sing-box 1.14 destination address groups combine domains and IP CIDRs with OR.
// Exercise both sides through its real rule constructor to guard the conversion.
func TestImportedSingboxDomainAndIPPreserveEngineOR(t *testing.T) {
	input := `[{"domain":"example.com","ip_cidr":"203.0.113.0/24","port":443,"outbound":"n1"}]`
	preview, err := Import("singbox", input, importSpec())
	if err != nil || len(preview.Rules) != 1 {
		t.Fatalf("preview: %+v %v", preview, err)
	}
	compiled, err := Build(importSpec(), preview.Rules[0])
	if err != nil {
		t.Fatal(err)
	}
	var original []map[string]any
	if err := json.Unmarshal([]byte(input), &original); err != nil {
		t.Fatal(err)
	}
	cases := []matchCase{
		{"", "example.com", "tcp", 443, true}, {"", "203.0.113.8", "tcp", 443, true},
		{"", "other.example", "tcp", 443, false}, {"", "203.0.113.8", "tcp", 80, false},
	}
	matchCompiled(t, original[0], cases)
	matchCompiled(t, compiled, cases)
}
