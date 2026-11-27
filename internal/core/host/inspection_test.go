package host

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"ferry/internal/spec"
	"github.com/sagernet/sing-box/log"
)

func TestInspectionUsesPrivateControllerAndKnownNodes(t *testing.T) {
	var closed bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-secret" {
			t.Error("missing controller authorization")
		}
		switch {
		case strings.HasSuffix(r.URL.Path, "/delay"):
			fmt.Fprint(w, `{"delay":42}`)
		case r.Method == "DELETE":
			closed = true
		case r.URL.Path == "/connections":
			fmt.Fprint(w, `{"connections":[{"id":"11111111-2222-4333-8444-555555555555","metadata":{"host":"example.com","destinationPort":"443","processPath":"C:\\Apps\\test.exe"},"upload":12,"download":34,"start":"2026-01-01T00:00:00Z","chains":["test"],"rule":"final"}]}`)
		}
	}))
	defer server.Close()
	h := New()
	h.current = &instance{address: strings.TrimPrefix(server.URL, "http://"), secret: "test-secret", client: server.Client(), nodes: map[string]bool{"test": true}}
	ctx := context.Background()
	delays, err := h.LatencyTest(ctx, []string{"test"})
	if err != nil || delays["test"] != 42 {
		t.Fatalf("latency: %v %v", delays, err)
	}
	if _, err := h.LatencyTest(ctx, []string{"../configs"}); err == nil {
		t.Fatal("unknown proxy identifier accepted")
	}
	connections, err := h.Connections(ctx)
	if err != nil || len(connections) != 1 || connections[0].Port != 443 || connections[0].Up != 12 || connections[0].Process != "test.exe" {
		t.Fatalf("connections: %+v %v", connections, err)
	}
	if err := h.CloseConnection(ctx, connections[0].ID); err != nil || !closed {
		t.Fatal("connection was not closed")
	}
	if err := h.CloseConnection(ctx, "../configs"); err == nil {
		t.Fatal("unsafe connection identifier accepted")
	}
}

func TestLogCaptureDoesNotEnablePersistentCoreCache(t *testing.T) {
	if !hasClashAPI {
		t.Skip("full build tags required")
	}
	t.Chdir(t.TempDir())
	s := spec.Default()
	s.Nodes = []spec.Node{{ID: "test", Tag: "test", Outbound: []byte(`{"type":"socks","server":"127.0.0.1","server_port":1080}`)}}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	s.Inbound.MixedPort = uint16(listener.Addr().(*net.TCPAddr).Port)
	listener.Close()
	h := New()
	if err := h.Start(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	if err := h.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat("cache.db"); !os.IsNotExist(err) {
		t.Fatal("log capture enabled an implicit core cache file")
	}
}

func TestLogBufferRedactsCredentialsAndBoundsHistory(t *testing.T) {
	s := spec.Default()
	s.Nodes = []spec.Node{{ID: "test", Tag: "test", Outbound: []byte(`{"type":"trojan","server":"example.com","server_port":443,"password":"synthetic-password","tls":{"enabled":true}}`)}}
	buffer := newLogBuffer()
	buffer.configure(s)
	for range 5010 {
		buffer.WriteMessage(log.LevelInfo, "request synthetic-password https://example.com/private-token")
	}
	entries := buffer.snapshot(0, 200)
	if len(entries) != 200 || entries[0].Seq != 11 || strings.Contains(entries[0].Message, "synthetic-password") || strings.Contains(entries[0].Message, "private-token") {
		t.Fatal("log retention or redaction failed")
	}
	if tail := buffer.snapshot(entries[199].Seq, 200); len(tail) != 200 || tail[0].Seq != 211 {
		t.Fatal("incremental log cursor failed")
	}
}
