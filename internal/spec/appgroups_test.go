package spec

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

type appGroupFixture struct {
	ID   string   `json:"id"`
	Name string   `json:"name"`
	Apps []string `json:"apps"`
}

func decodeAppGroups(groups []appGroupFixture, apps []string) (RunSpec, error) {
	payload := map[string]any{"appGroups": groups}
	if apps != nil {
		payload["rules"] = []Rule{{ID: "r", Name: "应用规则", Enabled: true, Apps: apps, Hosts: []string{}, Ports: []string{}, Network: "any", Action: "direct"}}
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return RunSpec{}, err
	}
	return Decode(data)
}

func TestDecodeAppGroupsSupportsReferencesAndOldConfigurations(t *testing.T) {
	groups := []appGroupFixture{{"browsers", "浏览器", []string{"chrome.exe", `C:\Apps (1)\firefox?.exe`}}}
	s, err := decodeAppGroups(groups, []string{"@group:browsers", "edge.exe"})
	if err != nil {
		t.Fatalf("valid group reference rejected: %v", err)
	}
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Decode(data); err != nil {
		t.Fatalf("group configuration cannot round trip: %v", err)
	}
	for _, payload := range []string{`{}`, `{"appGroups":[]}`} {
		legacy, err := Decode([]byte(payload))
		if err != nil {
			t.Fatalf("legacy configuration rejected: %v", err)
		}
		encoded, err := json.Marshal(legacy)
		if err != nil || bytes.Contains(encoded, []byte(`"appGroups"`)) {
			t.Fatalf("empty app groups changed the legacy wire format: %s, %v", encoded, err)
		}
	}
}

func TestAppGroupValidationRejectsUnsafeOrUnboundedMembers(t *testing.T) {
	for _, apps := range [][]string{
		{"@group:SECRET"}, {"@SECRET"}, {""}, {" chrome.exe"}, {"chrome.exe\n"},
		{`C:SECRET.exe`}, {`regex:SECRET`}, {`C:\SECRET.exe:stream`}, {`"SECRET"`},
		{strings.Repeat("x", 1025)}, make([]string, 257),
	} {
		_, err := decodeAppGroups([]appGroupFixture{{"browsers", "浏览器", apps}}, nil)
		if err == nil || strings.Contains(err.Error(), "SECRET") {
			t.Fatalf("invalid members accepted or leaked in error: %v", err)
		}
	}
	for _, groups := range [][]appGroupFixture{
		{{"", "浏览器", []string{}}}, {{"bad:id", "浏览器", []string{}}}, {{"g", " ", []string{}}},
		{{"g", "浏览器", []string{}}, {"g", "游戏", []string{}}},
	} {
		if _, err := decodeAppGroups(groups, nil); err == nil {
			t.Fatal("invalid or duplicate group accepted")
		}
	}
}

func TestAppGroupReferenceMustExistAndBeNonEmpty(t *testing.T) {
	groups := []appGroupFixture{{"empty", "空分组", []string{}}, {"valid", "有效", []string{"chrome.exe"}}}
	if _, err := decodeAppGroups(groups, nil); err != nil {
		t.Fatalf("unreferenced empty group rejected: %v", err)
	}
	for _, app := range []string{"@group:missing", "@group:empty", "@group:", "@group:valid:SECRET", "@valid"} {
		if _, err := decodeAppGroups(groups, []string{app}); err == nil || strings.Contains(err.Error(), "SECRET") {
			t.Fatalf("invalid reference accepted or leaked in error: %v", err)
		}
	}
}

func TestAppGroupCountAndMemberLimits(t *testing.T) {
	groups := make([]appGroupFixture, 128)
	for i := range groups {
		groups[i] = appGroupFixture{fmt.Sprintf("g%d", i), "分组", []string{}}
	}
	groups[0].Apps = make([]string, 256)
	for i := range groups[0].Apps {
		groups[0].Apps[i] = fmt.Sprintf("app%d.exe", i)
	}
	if _, err := decodeAppGroups(groups, []string{"@group:g0"}); err != nil {
		t.Fatalf("boundary configuration rejected: %v", err)
	}
	groups = append(groups, appGroupFixture{"overflow", "超限", []string{}})
	if _, err := decodeAppGroups(groups, nil); err == nil {
		t.Fatal("too many app groups accepted")
	}
}

func TestAppGroupDecodeRejectsUnknownFields(t *testing.T) {
	for _, payload := range []string{
		`{"AppGroups":[]}`,
		`{"appGroups":[{"id":"g","name":"G","Apps":["chrome.exe"]}]}`,
		`{"appGroups":[{"id":"g","name":"G","apps":[],"path":"SECRET"}]}`,
		`{"appGroups":[{"id":"g","name":"G","apps":["chrome.exe"],"apps":[]}]}`,
	} {
		if _, err := Decode([]byte(payload)); err == nil || strings.Contains(err.Error(), "SECRET") {
			t.Fatalf("invalid group JSON accepted or leaked: %v", err)
		}
	}
}

func expandedGroups(count int) []AppGroup {
	groups := make([]AppGroup, count)
	for i := range groups {
		groups[i] = AppGroup{ID: fmt.Sprintf("g%d", i), Name: "应用组", Apps: make([]string, 256)}
		for j := range groups[i].Apps {
			groups[i].Apps[j] = fmt.Sprintf("group%d-app%d.exe", i, j)
		}
	}
	return groups
}

func TestAppGroupExpansionBoundsUniqueConditionsPerRule(t *testing.T) {
	s := Default()
	s.AppGroups = expandedGroups(5)
	r := Rule{ID: "r", Name: "分组规则", Enabled: true, Network: "any", Action: "direct",
		Apps: []string{"@group:g0", "@group:g1", "@group:g2", "@group:g3"}}
	if err := s.ValidateRule(r); err != nil {
		t.Fatalf("1024 expanded conditions rejected: %v", err)
	}
	r.Apps = append(r.Apps, "@group:g0")
	if err := s.ValidateRule(r); err != nil {
		t.Fatalf("duplicate members counted more than once: %v", err)
	}
	r.Apps = append(r.Apps, "@group:g4")
	if err := s.ValidateRule(r); err == nil {
		t.Fatal("expansion beyond 1024 conditions accepted")
	}
}

func TestAppGroupExpansionBoundsTotalCompiledConditions(t *testing.T) {
	s := Default()
	s.AppGroups = expandedGroups(4)
	s.Rules = make([]Rule, 16)
	for i := range s.Rules {
		s.Rules[i] = Rule{ID: fmt.Sprintf("r%d", i), Name: "分组规则", Enabled: true, Network: "any", Action: "direct",
			Apps: []string{"@group:g0", "@group:g1", "@group:g2", "@group:g3"}}
	}
	if err := s.Validate(); err != nil {
		t.Fatalf("16384 total conditions rejected: %v", err)
	}
	s.Rules = append(s.Rules, Rule{ID: "overflow", Name: "超限", Enabled: true, Network: "any", Action: "direct", Apps: []string{"extra.exe"}})
	if err := s.Validate(); err == nil {
		t.Fatal("configuration expansion beyond 16384 conditions accepted")
	}
}
