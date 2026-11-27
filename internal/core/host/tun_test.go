package host

import (
	"context"
	"encoding/json"
	"ferry/internal/spec"
	"strings"
	"testing"
)

func TestUserModeHostRejectsTunAndServiceHostSetsServiceMode(t *testing.T) {
	cfg := spec.Default()
	cfg.Nodes = []spec.Node{{ID: "n1", Tag: "Node", Outbound: json.RawMessage(`{"type":"socks","server":"127.0.0.1","server_port":1080}`)}}
	cfg.Inbound.Tun.Enabled = true

	h := New()
	err := h.Start(context.Background(), cfg)
	if err == nil || !strings.Contains(err.Error(), "TUN") {
		t.Fatalf("expected user-mode host to reject TUN, got: %v", err)
	}
	if h.Status().ServiceMode {
		t.Fatal("user-mode host must not claim ServiceMode=true")
	}

	svcHost := NewService(t.TempDir())
	if !svcHost.Status().ServiceMode {
		t.Fatal("service host must report ServiceMode=true")
	}
}
