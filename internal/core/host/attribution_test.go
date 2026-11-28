package host

import (
	"context"
	"net"
	"testing"

	"ferry/internal/spec"
	F "github.com/sagernet/sing/common/format"
)

func TestBuildRuleLookupAttributesFerryRules(t *testing.T) {
	if !hasClashAPI {
		t.Skip("requires full build tags")
	}
	s := spec.Default()
	s.Nodes = []spec.Node{{ID: "n1", Tag: "n1", Outbound: []byte(`{"type":"socks","server":"127.0.0.1","server_port":1080}`)}}
	s.Rules = append(spec.DefaultRules()[:1],
		spec.Rule{ID: "r-cursor", Name: "Cursor 代理", Enabled: true, Apps: []string{"Cursor.exe"}, Hosts: []string{}, Ports: []string{}, Network: "any", Action: "proxy", TargetID: "proxy"},
		spec.DefaultRules()[1],
	)
	probe, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	s.Inbound.MixedPort = uint16(probe.Addr().(*net.TCPAddr).Port)
	probe.Close()

	h := New()
	if err := h.Start(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	defer h.Stop(context.Background())

	inst := h.active()
	if inst == nil || len(inst.ruleLookup) < 3 {
		t.Fatalf("ruleLookup not populated: %+v", inst)
	}
	var cursorKey string
	for _, r := range inst.box.Router().Rules() {
		key := F.ToString(r, " => ", r.Action())
		if meta, ok := inst.ruleLookup[key]; ok && meta.RuleID == "r-cursor" {
			cursorKey = key
			break
		}
	}
	if cursorKey == "" {
		t.Fatalf("failed to map compiled rule back to r-cursor: %+v", inst.ruleLookup)
	}
	raw := rawConnection{ID: "11111111-2222-4333-8444-555555555555", Rule: cursorKey, Chains: []string{"proxy"}}
	raw.Metadata.DestinationPort = "443"
	raw.Metadata.Host = "api2.cursor.sh"
	raw.Metadata.ProcessPath = `C:\Users\dev\Cursor.exe`
	raw.Metadata.Network = "tcp"
	mapped, err := mapRawConnection(raw, inst.ruleLookup)
	if err != nil {
		t.Fatal(err)
	}
	if mapped.RuleID != "r-cursor" || mapped.RuleName != "Cursor 代理" || mapped.RuleDetail != cursorKey || mapped.Network != "tcp" {
		t.Fatalf("attribution mismatch: %+v", mapped)
	}
}
