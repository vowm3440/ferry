package host

import (
	"bytes"
	"context"
	"net"
	"os"
	"path/filepath"
	"testing"

	"ferry/internal/spec"
	"ferry/internal/telemetry"
	"github.com/sagernet/sing-box/common/srs"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
)

func hostRuleSet(t *testing.T, root, id string, rule option.DefaultHeadlessRule) {
	t.Helper()
	var b bytes.Buffer
	if err := srs.Write(&b, option.PlainRuleSet{Rules: []option.HeadlessRule{{Type: "default", DefaultOptions: rule}}}, C.RuleSetVersionCurrent); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, id+".srs"), b.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestRuleSetsUseOfflineCoreMatcherAndPreserveOuterConditions(t *testing.T) {
	root := t.TempDir()
	hostRuleSet(t, root, "geosite-cn", option.DefaultHeadlessRule{Domain: []string{"example.cn"}})
	hostRuleSet(t, root, "geoip-cn", option.DefaultHeadlessRule{IPCIDR: []string{"203.0.113.0/24"}})
	h := NewWithRuleSets(root)
	s := spec.Default()
	s.Rules = []spec.Rule{{ID: "set", Name: "set", Enabled: true, Hosts: []string{"geosite:cn", "geoip:cn"}, Ports: []string{"443"}, Network: "tcp", Action: "block", ResolveBefore: true}}
	for _, tc := range []struct {
		host          string
		port          int
		wantID, stage string
	}{
		{"example.cn", 443, "set", "matched"}, {"203.0.113.5", 443, "set", "matched"}, {"example.cn", 80, "builtin-default", "matched"}, {"unresolved.example", 443, "set", "dns-required"},
	} {
		result, err := h.TestRule(context.Background(), telemetry.RuleTestRequest{Config: s, Host: tc.host, Port: tc.port, Network: "tcp"})
		if err != nil || result.MatchedRuleID != tc.wantID || result.Stage != tc.stage {
			t.Fatalf("%s:%d => %+v %v", tc.host, tc.port, result, err)
		}
	}
}

func TestCoreStartsFromCachedRuleSetsWithoutNetwork(t *testing.T) {
	if !hasClashAPI {
		t.Skip("requires full tags")
	}
	root := t.TempDir()
	hostRuleSet(t, root, "geosite-cn", option.DefaultHeadlessRule{Domain: []string{"example.cn"}})
	s := spec.Default()
	s.Nodes = []spec.Node{{ID: "n", Tag: "n", Outbound: []byte(`{"type":"socks","server":"127.0.0.1","server_port":1080}`)}}
	s.Rules = []spec.Rule{{ID: "set", Name: "set", Enabled: true, Hosts: []string{"geosite:cn"}, Network: "any", Action: "block"}}
	probe, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	s.Inbound.MixedPort = uint16(probe.Addr().(*net.TCPAddr).Port)
	probe.Close()
	h := NewWithRuleSets(root)
	defer h.Stop(context.Background())
	if err := h.Start(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	if h.Status().Status != "running" {
		t.Fatal("offline core did not start")
	}
}
