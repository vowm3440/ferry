//go:build windows

package ipc

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"sync/atomic"
	"testing"
	"time"
)

func TestCoreProcessEndToEnd(t *testing.T) {
	executable := os.Getenv("FERRY_TEST_CORE")
	if executable == "" {
		t.Skip("run scripts/test.ps1 to build and exercise the core subprocess")
	}
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "ferry-proxy-ok") }))
	defer target.Close()
	upstream, hits := testSOCKS(t)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()
	config := json.RawMessage(fmt.Sprintf(`{"nodes":[{"id":"local","tag":"local-test","outbound":{"type":"socks","server":"127.0.0.1","server_port":%d}}],"groups":[],"rules":[],"inbound":{"mixedPort":%d,"allowLan":false,"tun":{"enabled":false}},"mode":"global"}`, upstream, port))
	p := NewProcess(executable)
	defer p.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err = p.Start(ctx, config); err != nil {
		t.Fatal(err)
	}
	state, err := p.Status(ctx)
	if err != nil || state.Status != "running" {
		t.Fatalf("status %+v %v", state, err)
	}
	proxyURL, _ := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", port))
	transport := &http.Transport{Proxy: http.ProxyURL(proxyURL), DisableKeepAlives: true}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 5 * time.Second}
	response, err := client.Get(target.URL)
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(response.Body)
	response.Body.Close()
	if err != nil || string(data) != "ferry-proxy-ok" || hits.Load() != 1 {
		t.Fatalf("request missed SOCKS node: %q, hits=%d, %v", data, hits.Load(), err)
	}
	if err = p.SetMode(ctx, "direct"); err != nil {
		t.Fatal(err)
	}
	response, err = client.Get(target.URL)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if hits.Load() != 1 {
		t.Fatal("direct mode still used proxy node")
	}
	if _, err = p.Snapshot(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err = p.Connections(ctx); err != nil {
		t.Fatal(err)
	}
	if logs, logErr := p.Logs(ctx, 0, 200); logErr != nil || len(logs) == 0 {
		t.Fatalf("missing real core logs: %v", logErr)
	}
	if err = p.SetLogLevel(ctx, "debug"); err != nil {
		t.Fatal(err)
	}
	if _, err = p.LatencyTest(ctx, []string{"missing"}); err == nil {
		t.Fatal("unknown latency node accepted")
	}
	if err = p.CloseConnection(ctx, "invalid"); err == nil {
		t.Fatal("invalid connection ID accepted")
	}
	if err = p.CloseConnection(ctx, ""); err != nil {
		t.Fatal(err)
	}
	if err = p.Stop(ctx); err != nil {
		t.Fatal(err)
	}
	if err = p.Close(); err != nil {
		t.Fatal(err)
	}
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 300*time.Millisecond)
	if err == nil {
		conn.Close()
		t.Fatal("mixed listener survived process shutdown")
	}
}

func testSOCKS(t *testing.T) (int, *atomic.Int32) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { listener.Close() })
	hits := &atomic.Int32{}
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go proxySOCKS(conn, hits)
		}
	}()
	return listener.Addr().(*net.TCPAddr).Port, hits
}

func proxySOCKS(conn net.Conn, hits *atomic.Int32) {
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(8 * time.Second))
	var greeting [2]byte
	if _, err := io.ReadFull(conn, greeting[:]); err != nil || greeting[0] != 5 {
		return
	}
	methods := make([]byte, int(greeting[1]))
	if _, err := io.ReadFull(conn, methods); err != nil {
		return
	}
	if _, err := conn.Write([]byte{5, 0}); err != nil {
		return
	}
	address, err := readSOCKSAddress(conn)
	if err != nil {
		return
	}
	upstream, err := net.DialTimeout("tcp", address, 3*time.Second)
	if err != nil {
		return
	}
	defer upstream.Close()
	hits.Add(1)
	if _, err = conn.Write([]byte{5, 0, 0, 1, 127, 0, 0, 1, 0, 0}); err != nil {
		return
	}
	done := make(chan struct{})
	go func() { io.Copy(upstream, conn); upstream.Close(); close(done) }()
	io.Copy(conn, upstream)
	conn.Close()
	<-done
}

func readSOCKSAddress(conn net.Conn) (string, error) {
	var header [4]byte
	if _, err := io.ReadFull(conn, header[:]); err != nil {
		return "", err
	}
	if header[0] != 5 || header[1] != 1 {
		return "", fmt.Errorf("unsupported SOCKS request")
	}
	var host string
	switch header[3] {
	case 1:
		var ip [4]byte
		if _, err := io.ReadFull(conn, ip[:]); err != nil {
			return "", err
		}
		host = net.IP(ip[:]).String()
	case 4:
		var ip [16]byte
		if _, err := io.ReadFull(conn, ip[:]); err != nil {
			return "", err
		}
		host = net.IP(ip[:]).String()
	case 3:
		var size [1]byte
		if _, err := io.ReadFull(conn, size[:]); err != nil {
			return "", err
		}
		name := make([]byte, int(size[0]))
		if _, err := io.ReadFull(conn, name); err != nil {
			return "", err
		}
		host = string(name)
	default:
		return "", fmt.Errorf("invalid SOCKS address")
	}
	var port [2]byte
	if _, err := io.ReadFull(conn, port[:]); err != nil {
		return "", err
	}
	if host != "127.0.0.1" && host != "::1" {
		return "", fmt.Errorf("test only permits loopback")
	}
	return net.JoinHostPort(host, strconv.Itoa(int(binary.BigEndian.Uint16(port[:])))), nil
}

func TestProcessRefusesMissingExecutable(t *testing.T) {
	p := NewProcess(`C:\definitely-missing\ferry-core.exe`)
	defer p.Close()
	if err := p.Start(context.Background(), json.RawMessage(`{}`)); err == nil {
		t.Fatal("missing executable accepted")
	}
	p = NewProcess(`C:\foreign.exe`)
	if err := p.Start(context.Background(), json.RawMessage(`{}`)); err == nil {
		t.Fatal("arbitrary executable accepted")
	}
	if _, err := p.Status(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Snapshot(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := p.SetMode(context.Background(), "rule"); err == nil {
		t.Fatal("mode without process accepted")
	}
	if err := p.SelectNode(context.Background(), "g", "n"); err == nil {
		t.Fatal("selection without process accepted")
	}
	if err := p.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
}
