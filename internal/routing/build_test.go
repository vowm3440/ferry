package routing

import (
	"context"
	"encoding/json"
	"net"
	"strconv"
	"testing"

	"ferry/internal/spec"
	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	sbrule "github.com/sagernet/sing-box/route/rule"
	corejson "github.com/sagernet/sing/common/json"
	M "github.com/sagernet/sing/common/metadata"
)

type matchCase struct {
	app, host, network string
	port               int
	want               bool
}

func matchCompiled(t *testing.T, value map[string]any, cases []matchCase) {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var options option.Rule
	if err := corejson.UnmarshalContext(context.Background(), data, &options); err != nil {
		t.Fatal(err)
	}
	rule, err := sbrule.NewRule(context.Background(), log.NewNOPFactory().NewLogger("test"), options, false)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = rule.Close() })
	for _, tc := range cases {
		metadata := &adapter.InboundContext{Network: tc.network, Domain: tc.host,
			Destination: M.ParseSocksaddr(net.JoinHostPort(tc.host, strconv.Itoa(tc.port))),
			ProcessInfo: &adapter.ConnectionOwner{ProcessPath: tc.app}}
		if actual := rule.Match(metadata); actual != tc.want {
			t.Errorf("match %s/%s/%s:%d = %t, want %t", tc.app, tc.host, tc.network, tc.port, actual, tc.want)
		}
	}
}

func TestCompiledRulesMatchWithSingboxEngine(t *testing.T) {
	r := spec.Rule{ID: "r1", Name: "browser", Enabled: true, Apps: []string{"chrome.exe", `*\Games\*.exe`},
		Hosts: []string{"*.example.com", "203.0.113.1-203.0.113.10"}, Ports: []string{"443", "8000-9000"},
		Network: "tcp", Action: "proxy", TargetID: "proxy"}
	value, err := Build(spec.Default(), r)
	if err != nil {
		t.Fatal(err)
	}
	matchCompiled(t, value, []matchCase{
		{`C:\Apps\CHROME.EXE`, "api.example.com", "tcp", 443, true},
		{`D:\Games\game.exe`, "203.0.113.7", "tcp", 8080, true},
		{`C:\Apps\other.exe`, "api.example.com", "tcp", 443, false},
		{`C:\Apps\chrome.exe`, "api.example.com", "udp", 443, false},
		{`C:\Apps\chrome.exe`, "api.example.com", "tcp", 444, false},
		{`C:\Apps\chrome.exe`, "203.0.113.11", "tcp", 443, false},
		{`C:\Apps\chrome.exe`, "203.0.113.0", "tcp", 443, false},
		{`C:\Apps\chrome.exe`, "evil-example.com", "tcp", 443, false},
	})
}

func TestProcessGlobEscapesAndAnchors(t *testing.T) {
	r := spec.Rule{ID: "r", Name: "escaped", Enabled: true, Network: "any", Action: "direct",
		Apps: []string{`C:\Apps (x86)\a+b.exe`, "chrome.exe", `*\Games\game?.exe`}}
	value, err := Build(spec.Default(), r)
	if err != nil {
		t.Fatal(err)
	}
	matchCompiled(t, value, []matchCase{
		{`c:\apps (X86)\A+B.EXE`, "test.example", "tcp", 443, true},
		{`C:\Apps x86\ab.exe`, "test.example", "tcp", 443, false},
		{`D:\Apps (x86)\a+b.exe`, "test.example", "tcp", 443, false},
		{`C:\Apps\notchrome.exe`, "test.example", "tcp", 443, false},
		{`C:\Apps\chrome.exe.bak`, "test.example", "tcp", 443, false},
		{`chrome.EXE`, "test.example", "tcp", 443, true},
		{`D:\Games\game1.exe`, "test.example", "udp", 1, true},
		{`D:\Games\game12.exe`, "test.example", "udp", 1, false},
	})
}

func TestHostAlternativesAndIPv6Ranges(t *testing.T) {
	r := spec.Rule{ID: "r", Name: "hosts", Enabled: true, Network: "any", Action: "block",
		Hosts: []string{"EXAMPLE.com", "*.suffix.test", "cdn?.asset-*.test", "192.0.2.19/24", "2001:db8::3-2001:db8::9"}}
	value, err := Build(spec.Default(), r)
	if err != nil {
		t.Fatal(err)
	}
	matchCompiled(t, value, []matchCase{
		{"", "example.com", "tcp", 80, true}, {"", "www.example.com", "tcp", 80, false},
		{"", "www.suffix.test", "tcp", 80, true}, {"", "suffix.test", "tcp", 80, false},
		{"", "cdn1.asset-img.test", "tcp", 80, true}, {"", "cdn11.asset-img.test", "tcp", 80, false},
		{"", "cdn1Xasset-imgXtest", "tcp", 80, false}, {"", "192.0.2.255", "tcp", 80, true},
		{"", "192.0.3.0", "tcp", 80, false}, {"", "2001:db8::3", "udp", 53, true},
		{"", "2001:db8::9", "udp", 53, true}, {"", "2001:db8::a", "udp", 53, false},
		{"", "2001:db8::2", "udp", 53, false},
	})
}

func TestRuleActionsMapToCompiledTags(t *testing.T) {
	s := spec.Default()
	s.Nodes = []spec.Node{{ID: "node", Tag: "display"}}
	s.Groups = []spec.Group{{ID: "group"}}
	for _, tc := range []struct{ action, target, outbound, compiled string }{
		{"direct", "", "__direct", "route"}, {"block", "", "", "reject"},
		{"proxy", "node", "node", "route"}, {"proxy", "group", "group", "route"},
		{"proxy", "proxy", "group", "route"}, {"proxy", "", "group", "route"},
	} {
		r := spec.Rule{ID: "r", Name: "R", Enabled: true, Network: "any", Action: tc.action, TargetID: tc.target}
		value, err := Build(s, r)
		if err != nil || value["action"] != tc.compiled || (tc.outbound != "" && value["outbound"] != tc.outbound) {
			t.Fatalf("action %s/%s: %v, %v", tc.action, tc.target, value, err)
		}
	}
}

func TestResolveOnlyForPotentialIPMatch(t *testing.T) {
	r := spec.Rule{ID: "r", Name: "IP", Enabled: true, Network: "tcp", Action: "proxy", ResolveBefore: true,
		Apps: []string{"chrome.exe"}, Ports: []string{"443"}, Hosts: []string{"example.com", "203.0.113.0/24"}}
	value, err := BuildResolve(spec.Default(), r)
	if err != nil || value["action"] != "resolve" || value["server"] != "__dns_remote" {
		t.Fatalf("resolve: %v, %v", value, err)
	}
	matchCompiled(t, value, []matchCase{
		{"chrome.exe", "other.example", "tcp", 443, true}, {"other.exe", "other.example", "tcp", 443, false},
		{"chrome.exe", "other.example", "udp", 443, false}, {"chrome.exe", "other.example", "tcp", 80, false},
		{"chrome.exe", "example.com", "tcp", 443, false},
	})
	r.ResolveBefore = false
	if value, err := BuildResolve(spec.Default(), r); err != nil || value != nil {
		t.Fatalf("disabled resolve: %v, %v", value, err)
	}
	r.ResolveBefore, r.Hosts = true, []string{"example.com"}
	if value, err := BuildResolve(spec.Default(), r); err != nil || value != nil {
		t.Fatalf("domain-only resolve: %v, %v", value, err)
	}
}
