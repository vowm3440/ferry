//go:build with_gvisor

package compile

import (
	"context"
	"encoding/json"
	include "ferry/internal/core/boxinclude"
	"ferry/internal/spec"
	box "github.com/sagernet/sing-box"
	"testing"
)

func TestTunCompileConstructsWithRealCore(t *testing.T) {
	cfg := spec.Default()
	cfg.Nodes = []spec.Node{{ID: "n1", Tag: "Node1", Outbound: json.RawMessage(`{"type":"socks","server":"127.0.0.1","server_port":1080}`)}}
	cfg.Inbound.Tun = spec.Tun{
		Enabled:     true,
		Stack:       "mixed",
		StrictRoute: true,
		Excludes:    []string{"10.0.0.0/8", "192.168.0.0/16"},
		MTU:         9000,
	}
	options, err := Compile(cfg)
	if err != nil {
		t.Fatalf("TUN compile failed: %v", err)
	}
	if len(options.Inbounds) != 2 || !options.Route.AutoDetectInterface {
		t.Fatalf("expected 2 inbounds and auto_detect_interface=true: inbounds=%d", len(options.Inbounds))
	}
	inst, err := box.New(box.Options{Options: options, Context: include.Context(context.Background())})
	if err != nil {
		t.Fatalf("TUN options rejected by sing-box constructor: %v", err)
	}
	if err := inst.Close(); err != nil {
		t.Fatal(err)
	}
}
