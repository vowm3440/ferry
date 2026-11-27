//go:build windows

package app

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"ferry/internal/ipc"
)

func TestImportedSubscriptionCoreProcess(t *testing.T) {
	session, process, port := importProcessSession(t)
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, "direct")
	}))
	defer target.Close()
	link, hits := importSOCKSNode(t, strings.TrimPrefix(target.URL, "http://"))
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	sub, err := session.Subs().AddFromText(ctx, "local-import-test", link)
	if err != nil {
		t.Fatal(err)
	}
	core, _, _ := session.Services()
	if err := core.SetMode(ctx, "global"); err != nil {
		t.Fatal(err)
	}
	if err := core.Start(ctx); err != nil {
		t.Fatalf("imported link cannot start the real core: %v", err)
	}
	client := importProxyClient(t, port)
	if body := importRequest(t, client, target.URL); body != "via-imported-node" || hits.Load() != 1 {
		t.Fatalf("request did not traverse the imported node: hits=%d", hits.Load())
	}
	if err := core.SetMode(ctx, "direct"); err != nil {
		t.Fatal(err)
	}
	if body := importRequest(t, client, target.URL); body != "direct" || hits.Load() != 1 {
		t.Fatal("direct mode still traversed the imported node")
	}
	if err := core.Restart(ctx); err != nil {
		t.Fatal(err)
	}
	if err := core.SetMode(ctx, "global"); err != nil {
		t.Fatal(err)
	}
	if body := importRequest(t, client, target.URL); body != "via-imported-node" || hits.Load() != 2 {
		t.Fatal("imported node did not survive restart")
	}
	if err := core.Stop(ctx); err != nil {
		t.Fatal(err)
	}
	if err := session.Subs().Remove(ctx, sub.ID); err != nil {
		t.Fatal(err)
	}
	if err := core.Start(ctx); err == nil {
		t.Fatal("core started after removing its only subscription")
	}
	assertImportProcessClosed(t, process, port)
}

// Opt-in live check. Credentials are read from an existing file outside the
// repository and never printed or included in assertions.
func TestImportedSubscriptionLiveNode(t *testing.T) {
	path := os.Getenv("FERRY_TEST_LINKS")
	if path == "" {
		t.Skip("set FERRY_TEST_LINKS to an external share-link file for a live test")
	}
	session, process, port := importProcessSession(t)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal("cannot read the external share-link file")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	sub, err := session.Subs().AddFromText(ctx, "live-test", string(data))
	if err != nil || sub.NodeCount == 0 {
		t.Fatalf("live link import failed: %v", err)
	}
	core, _, _ := session.Services()
	if err := core.SetMode(ctx, "global"); err != nil {
		t.Fatal(err)
	}
	if err := core.Start(ctx); err != nil {
		t.Fatal(err)
	}
	client := importProxyClient(t, port)
	client.Timeout = 20 * time.Second
	body := importRequest(t, client, "https://www.cloudflare.com/cdn-cgi/trace")
	if !strings.Contains(body, "ip=") || !strings.Contains(body, "loc=") {
		t.Fatal("live proxy did not return a valid HTTPS connectivity response")
	}
	if err := core.Stop(ctx); err != nil {
		t.Fatal(err)
	}
	assertImportProcessClosed(t, process, port)
}

func importProcessSession(t *testing.T) (*Session, *ipc.Process, int) {
	t.Helper()
	executable := os.Getenv("FERRY_TEST_CORE")
	if executable == "" {
		t.Skip("run scripts/test.ps1 to exercise the core subprocess")
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	process := ipc.NewProcess(executable)
	t.Cleanup(func() {
		if err := process.Close(); err != nil {
			t.Error(err)
		}
	})
	session := NewSession(process)
	_, _, settings := session.Services()
	value, err := settings.Get(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	value.MixedPort, value.Tun.FakeIP = port, false
	if err := settings.Set(context.Background(), value); err != nil {
		t.Fatal(err)
	}
	return session, process, port
}

func importProxyClient(t *testing.T, port int) *http.Client {
	t.Helper()
	proxy, err := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", port))
	if err != nil {
		t.Fatal(err)
	}
	transport := &http.Transport{Proxy: http.ProxyURL(proxy), DisableKeepAlives: true}
	t.Cleanup(transport.CloseIdleConnections)
	return &http.Client{Transport: transport, Timeout: 5 * time.Second}
}

func importRequest(t *testing.T, client *http.Client, target string) string {
	t.Helper()
	response, err := client.Get(target)
	if err != nil {
		t.Fatal("proxy connectivity request failed")
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 64<<10))
	if err != nil || response.StatusCode != http.StatusOK {
		t.Fatalf("proxy response failed: status=%d", response.StatusCode)
	}
	return string(body)
}

func assertImportProcessClosed(t *testing.T, process *ipc.Process, port int) {
	t.Helper()
	if err := process.Close(); err != nil {
		t.Fatal(err)
	}
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 300*time.Millisecond)
	if err == nil {
		conn.Close()
		t.Fatal("mixed inbound survived core process shutdown")
	}
}

func importSOCKSNode(t *testing.T, target string) (string, *atomic.Int32) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	hits := &atomic.Int32{}
	var workers sync.WaitGroup
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			workers.Add(1)
			go func() {
				defer workers.Done()
				serveImportSOCKS(conn, target, hits)
			}()
		}
	}()
	t.Cleanup(func() { listener.Close(); <-done; workers.Wait() })
	return "socks5://" + listener.Addr().String() + "#local", hits
}

func serveImportSOCKS(conn net.Conn, target string, hits *atomic.Int32) {
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		return
	}
	var greeting [2]byte
	if _, err := io.ReadFull(conn, greeting[:]); err != nil || greeting[0] != 5 {
		return
	}
	if _, err := io.CopyN(io.Discard, conn, int64(greeting[1])); err != nil {
		return
	}
	if _, err := conn.Write([]byte{5, 0}); err != nil {
		return
	}
	var connect [10]byte // The fixture target is always a literal IPv4 address.
	if _, err := io.ReadFull(conn, connect[:]); err != nil || connect[0] != 5 || connect[1] != 1 || connect[3] != 1 {
		return
	}
	address := fmt.Sprintf("%s:%d", net.IP(connect[4:8]), int(connect[8])<<8|int(connect[9]))
	if address != target {
		return
	}
	if _, err := conn.Write([]byte{5, 0, 0, 1, 127, 0, 0, 1, 0, 0}); err != nil {
		return
	}
	request, err := http.ReadRequest(bufio.NewReader(conn))
	if err != nil {
		return
	}
	defer request.Body.Close()
	if request.Host != target {
		return
	}
	hits.Add(1)
	const body = "via-imported-node"
	fmt.Fprintf(conn, "HTTP/1.1 200 OK\r\nContent-Length: %d\r\nConnection: close\r\n\r\n%s", len(body), body)
}
