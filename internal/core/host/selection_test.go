//go:build with_clash_api

package host

import (
	"context"
	"ferry/internal/spec"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSelectionChangesEgressWithoutRestart(t *testing.T) {
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, "ferry-real-egress") }))
	defer origin.Close()
	first, second := newSOCKS(t), newSOCKS(t)
	s := hostFixture(t, first.Addr().(*net.TCPAddr).Port)
	n2 := hostFixture(t, second.Addr().(*net.TCPAddr).Port).Nodes[0]
	n2.ID, n2.Tag = "n2", "第二节点"
	s.Nodes = append(s.Nodes, n2)
	s.Groups = []spec.Group{{ID: "g", Name: "手动选择", Type: "select", Members: []string{"n1", "n2"}}}
	s.Mode = "global"
	h := New()
	defer h.Stop(context.Background())
	if err := h.Start(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	running := h.active()
	requestThroughProxy(t, s.Inbound.MixedPort, origin.URL)
	if err := h.SelectNode(context.Background(), "g", "n2"); err != nil {
		t.Fatal(err)
	}
	requestThroughProxy(t, s.Inbound.MixedPort, origin.URL)
	if first.requests.Load() != 1 || second.requests.Load() != 1 {
		t.Fatal("切换未改变实际出站")
	}
	if h.active() != running {
		t.Fatal("节点切换重启了内核")
	}
	if err := h.SetMode(context.Background(), "direct"); err != nil {
		t.Fatal(err)
	}
	requestThroughProxy(t, s.Inbound.MixedPort, origin.URL)
	if first.requests.Load() != 1 || second.requests.Load() != 1 {
		t.Fatal("直连模式仍使用代理节点")
	}
}
