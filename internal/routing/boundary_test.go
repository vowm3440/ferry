package routing

import (
	"encoding/json"
	"testing"

	"ferry/internal/spec"
)

func TestBuildRejectsInvalidDisabledRuleAndIsPure(t *testing.T) {
	s := spec.Default()
	r := spec.Rule{ID: "r", Name: "R", Network: "tcp", Action: "proxy", TargetID: "missing"}
	if _, err := Build(s, r); err == nil {
		t.Fatal("invalid disabled rule accepted")
	}
	if _, err := BuildResolve(s, r); err == nil {
		t.Fatal("invalid resolve rule accepted")
	}
	r.Enabled, r.TargetID, r.Hosts = true, "proxy", []string{"203.0.113.4", "*.example.com"}
	r.Apps, r.Ports = []string{`C:/Apps/a.exe`}, []string{"1-65535"}
	s.Rules = []spec.Rule{r}
	before, _ := json.Marshal(s)
	value, err := Build(s, r)
	if err != nil {
		t.Fatal(err)
	}
	matchCompiled(t, value, []matchCase{
		{`C:\Apps\a.exe`, "203.0.113.4", "tcp", 1, true},
		{`C:\Apps\a.exe`, "203.0.113.4", "tcp", 65535, true},
		{`C:\Apps\a.exe`, "203.0.113.5", "tcp", 443, false},
	})
	after, _ := json.Marshal(s)
	if string(before) != string(after) {
		t.Fatal("Build mutated its input")
	}
}

func TestFullAddressRangesTerminateAndMatchBoundaries(t *testing.T) {
	for _, host := range []string{"0.0.0.0-255.255.255.255", "::-ffff:ffff:ffff:ffff:ffff:ffff:ffff:ffff"} {
		r := spec.Rule{ID: "r", Name: "R", Enabled: true, Network: "any", Action: "direct", Hosts: []string{host}}
		value, err := Build(spec.Default(), r)
		if err != nil {
			t.Fatal(err)
		}
		ipv4 := host[0] == '0'
		matchCompiled(t, value, []matchCase{
			{"", "0.0.0.0", "tcp", 1, ipv4}, {"", "255.255.255.255", "tcp", 1, ipv4},
			{"", "::", "tcp", 1, !ipv4}, {"", "ffff:ffff:ffff:ffff:ffff:ffff:ffff:ffff", "tcp", 1, !ipv4},
		})
	}
}

func TestRealProxyGroupIsAddressableWithoutChangingDefault(t *testing.T) {
	s := spec.Default()
	s.Groups = []spec.Group{{ID: "first"}, {ID: "proxy"}}
	user := spec.Rule{ID: "r", Name: "R", Enabled: true, Network: "any", Action: "proxy", TargetID: "proxy"}
	for _, tc := range []struct {
		rule spec.Rule
		want string
	}{
		{user, "proxy"}, {spec.DefaultRules()[1], "first"},
	} {
		value, err := Build(s, tc.rule)
		if err != nil || value["outbound"] != tc.want {
			t.Fatalf("rule %s: %v/%v, want %s", tc.rule.ID, value, err, tc.want)
		}
	}
	user.TargetID = ""
	value, err := Build(s, user)
	if err != nil || value["outbound"] != "first" {
		t.Fatalf("empty target: %v/%v", value, err)
	}
}
