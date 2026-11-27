package share

import (
	"context"
	"encoding/base64"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func newLoopbackLAN(t *testing.T, ttl time.Duration) *LAN {
	t.Helper()
	server, err := NewLAN(LANOptions{BindIP: "127.0.0.1", TTL: ttl})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := server.Close(); err != nil {
			t.Error(err)
		}
	})
	return server
}

func startLAN(t *testing.T, server *LAN) LANInfo {
	t.Helper()
	info, err := server.Start(context.Background(), []byte("private configuration"), "application/json")
	if err != nil {
		t.Fatal(err)
	}
	return info
}

func lanClient(t *testing.T) *http.Client {
	t.Helper()
	transport := &http.Transport{Proxy: nil, DisableKeepAlives: true}
	t.Cleanup(transport.CloseIdleConnections)
	return &http.Client{Transport: transport, Timeout: 2 * time.Second}
}

func readLAN(t *testing.T, client *http.Client, address, method string) (int, string, http.Header) {
	t.Helper()
	request, err := http.NewRequest(method, address, nil)
	if err != nil {
		t.Fatal(err)
	}
	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("request failed: %T", err)
	}
	defer response.Body.Close()
	data, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal("reading shared content failed")
	}
	return response.StatusCode, string(data), response.Header
}

func waitLANClosed(t *testing.T, info LANInfo) {
	t.Helper()
	address, err := url.Parse(info.URL)
	if err != nil {
		t.Fatal("invalid sharing URL")
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		connection, err := net.DialTimeout("tcp", address.Host, 20*time.Millisecond)
		if err != nil {
			return
		}
		connection.Close()
		time.Sleep(time.Millisecond)
	}
	t.Fatal("sharing port stayed open")
}

func TestLANStartsOnlyExplicitlyAndTransfersSnapshotOnce(t *testing.T) {
	server := newLoopbackLAN(t, 0)
	if info := server.Status(); info.Active || info.URL != "" {
		t.Fatal("sharing started during construction")
	}
	payload := []byte("private configuration")
	before := time.Now()
	info, err := server.Start(context.Background(), payload, "application/json")
	if err != nil || !info.Active {
		t.Fatalf("start failed: %v", err)
	}
	payload[0] = 'X'
	if info.ExpiresAt.Before(before.Add(9*time.Minute)) || info.ExpiresAt.After(time.Now().Add(10*time.Minute)) {
		t.Fatal("default expiration is not ten minutes")
	}
	parsed, _ := url.Parse(info.URL)
	token, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(parsed.Path, "/s/"))
	if err != nil || len(token) != 32 || parsed.RawQuery != "" || parsed.Hostname() != "127.0.0.1" {
		t.Fatal("sharing URL does not contain a 256-bit path token and actual bind address")
	}
	status, body, headers := readLAN(t, lanClient(t), info.URL, http.MethodGet)
	if status != http.StatusOK || body != "private configuration" {
		t.Fatal("sharing did not return its original snapshot")
	}
	if headers.Get("Content-Type") != "application/json" || headers.Get("Cache-Control") != "no-store" {
		t.Fatal("unsafe or incorrect content response headers")
	}
	waitLANClosed(t, info)
	if state := server.Status(); state.Active || state.URL != "" {
		t.Fatal("consumed sharing session remains active")
	}
}

func TestLANUnknownPathsAndMethodsDoNotRevealOrConsumeContent(t *testing.T) {
	for _, suffix := range []string{"/", "/s/wrong", "/../", "/favicon.ico"} {
		t.Run(suffix, func(t *testing.T) {
			server := newLoopbackLAN(t, 0)
			info := startLAN(t, server)
			parsed, _ := url.Parse(info.URL)
			status, body, _ := readLAN(t, lanClient(t), parsed.Scheme+"://"+parsed.Host+suffix, http.MethodGet)
			if status != http.StatusNotFound || strings.Contains(body, "private") {
				t.Fatal("unknown path exposed sharing content")
			}
			status, body, _ = readLAN(t, lanClient(t), info.URL, http.MethodGet)
			if status != http.StatusOK || body != "private configuration" {
				t.Fatal("unknown path consumed sharing token")
			}
		})
	}
	for _, method := range []string{http.MethodHead, http.MethodPost, http.MethodOptions} {
		t.Run(method, func(t *testing.T) {
			server := newLoopbackLAN(t, 0)
			info := startLAN(t, server)
			status, body, _ := readLAN(t, lanClient(t), info.URL, method)
			if status != http.StatusMethodNotAllowed || strings.Contains(body, "private") {
				t.Fatal("unsupported method exposed sharing content")
			}
			status, _, _ = readLAN(t, lanClient(t), info.URL, http.MethodGet)
			if status != http.StatusOK {
				t.Fatal("unsupported method consumed sharing token")
			}
		})
	}
}

func TestLANExpirationClosesListener(t *testing.T) {
	server := newLoopbackLAN(t, 30*time.Millisecond)
	info := startLAN(t, server)
	time.Sleep(time.Until(info.ExpiresAt) + 5*time.Millisecond)
	waitLANClosed(t, info)
	if server.Status().Active {
		t.Fatal("expired sharing remains active")
	}
}

func TestLANStopClosesConnectionsAndCanRestart(t *testing.T) {
	server := newLoopbackLAN(t, 0)
	first := startLAN(t, server)
	parsed, _ := url.Parse(first.URL)
	connection, err := net.DialTimeout("tcp", parsed.Host, time.Second)
	if err != nil {
		t.Fatal("cannot connect to sharing listener")
	}
	defer connection.Close()
	if _, err := io.WriteString(connection, "GET / HTTP/1.1\r\nHost: example\r\n"); err != nil {
		t.Fatal(err)
	}
	if err := server.Stop(); err != nil {
		t.Fatal(err)
	}
	connection.SetReadDeadline(time.Now().Add(time.Second))
	if _, err := connection.Read(make([]byte, 1)); err == nil {
		t.Fatal("stop retained an incomplete HTTP connection")
	}
	waitLANClosed(t, first)
	second := startLAN(t, server)
	if second.URL == first.URL {
		t.Fatal("restart reused sharing token")
	}
	if err := server.Close(); err != nil {
		t.Fatal(err)
	}
	waitLANClosed(t, second)
	if _, err := server.Start(context.Background(), []byte("x"), "text/plain"); err == nil {
		t.Fatal("permanently closed sharing server restarted")
	}
}

func TestLANParallelFetchHasOnlyOneSuccess(t *testing.T) {
	server := newLoopbackLAN(t, 0)
	info := startLAN(t, server)
	client := lanClient(t)
	var successes atomic.Int32
	var group sync.WaitGroup
	start := make(chan struct{})
	for range 24 {
		group.Go(func() {
			<-start
			response, err := client.Get(info.URL)
			if err != nil {
				return
			}
			defer response.Body.Close()
			data, err := io.ReadAll(response.Body)
			if response.StatusCode == http.StatusOK && err == nil && string(data) == "private configuration" {
				successes.Add(1)
			}
		})
	}
	close(start)
	group.Wait()
	if successes.Load() != 1 {
		t.Fatalf("successful downloads = %d, want one", successes.Load())
	}
	waitLANClosed(t, info)
}

func TestLANRejectsOversizeEmptyAndCanceledStart(t *testing.T) {
	server := newLoopbackLAN(t, 0)
	for _, payload := range [][]byte{nil, {}, make([]byte, (1<<20)+1)} {
		if _, err := server.Start(context.Background(), payload, "text/plain"); err == nil {
			t.Fatal("invalid content size accepted")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := server.Start(ctx, []byte("x"), "text/plain"); err != context.Canceled {
		t.Fatal("canceled start did not return cancellation")
	}
	if server.Status().Active {
		t.Fatal("failed start activated sharing")
	}
}
