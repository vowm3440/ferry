//go:build windows

package ipc

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"testing"
	"time"
)

func TestLogCursorSurvivesCoreProcessReplacement(t *testing.T) {
	executable := os.Getenv("FERRY_TEST_CORE")
	if executable == "" {
		t.Skip("requires built core subprocess")
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()
	config := json.RawMessage(fmt.Sprintf(`{"nodes":[{"id":"local","tag":"test","outbound":{"type":"socks","server":"127.0.0.1","server_port":1080}}],"inbound":{"mixedPort":%d}}`, port))
	p := NewProcess(executable)
	defer p.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err = p.Start(ctx, config); err != nil {
		t.Fatal(err)
	}
	entries, err := p.Logs(ctx, 0, 200)
	if err != nil || len(entries) == 0 {
		t.Fatal("missing initial logs")
	}
	last := entries[len(entries)-1].Seq
	if err = p.cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	<-p.done
	if err = p.Start(ctx, config); err != nil {
		t.Fatal(err)
	}
	entries, err = p.Logs(ctx, last, 200)
	if err != nil || len(entries) == 0 || entries[0].Seq <= last {
		t.Fatalf("new process logs hidden by old cursor: %d entries, %v", len(entries), err)
	}
}
