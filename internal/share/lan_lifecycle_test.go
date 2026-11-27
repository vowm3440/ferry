package share

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/url"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestLANParallelStartCreatesOnlyOneSession(t *testing.T) {
	server := newLoopbackLAN(t, 0)
	var started atomic.Int32
	var group sync.WaitGroup
	start := make(chan struct{})
	for range 24 {
		group.Go(func() {
			<-start
			if _, err := server.Start(context.Background(), []byte("snapshot"), "text/plain"); err == nil {
				started.Add(1)
			}
		})
	}
	close(start)
	group.Wait()
	if started.Load() != 1 {
		t.Fatalf("started sessions = %d, want one", started.Load())
	}
	info := server.Status()
	status, body, _ := readLAN(t, lanClient(t), info.URL, http.MethodGet)
	if status != http.StatusOK || body != "snapshot" {
		t.Fatal("concurrent starts left no readable snapshot")
	}
}

func TestLANConcurrentStartsStopsAndCloseLeaveNoListener(t *testing.T) {
	server := newLoopbackLAN(t, 0)
	var group sync.WaitGroup
	var mu sync.Mutex
	var infos []LANInfo
	for range 24 {
		group.Go(func() {
			info, err := server.Start(context.Background(), []byte("snapshot"), "text/plain")
			if err == nil {
				mu.Lock()
				infos = append(infos, info)
				mu.Unlock()
			}
			server.Status()
			if err := server.Stop(); err != nil {
				t.Error(err)
			}
		})
	}
	group.Go(func() {
		if err := server.Close(); err != nil {
			t.Error(err)
		}
	})
	group.Wait()
	for _, info := range infos {
		waitLANClosed(t, info)
	}
	if server.Status().Active {
		t.Fatal("closed sharing owner remains active")
	}
}

func TestLANExpirationClosesAlreadyConnectedClient(t *testing.T) {
	server := newLoopbackLAN(t, 50*time.Millisecond)
	info := startLAN(t, server)
	parsed, _ := url.Parse(info.URL)
	connection, err := net.DialTimeout("tcp", parsed.Host, time.Second)
	if err != nil {
		t.Fatal("cannot connect before expiration")
	}
	defer connection.Close()
	io.WriteString(connection, "GET "+parsed.EscapedPath()+" HTTP/1.1\r\n")
	connection.SetReadDeadline(time.Now().Add(time.Second))
	data, err := io.ReadAll(connection)
	if len(data) != 0 {
		t.Fatal("incomplete request received content after expiry")
	}
	if timeout, ok := err.(net.Error); ok && timeout.Timeout() {
		t.Fatal("expiration retained an existing connection")
	}
	waitLANClosed(t, info)
}

func TestLANReadHeaderDeadlineReleasesStalledClient(t *testing.T) {
	server := newLoopbackLAN(t, 0)
	info := startLAN(t, server)
	parsed, _ := url.Parse(info.URL)
	connection, err := net.DialTimeout("tcp", parsed.Host, time.Second)
	if err != nil {
		t.Fatal("cannot connect to sharing listener")
	}
	defer connection.Close()
	connection.SetReadDeadline(time.Now().Add(3 * time.Second))
	data, err := io.ReadAll(connection)
	if len(data) != 0 {
		t.Fatal("empty request received sharing content")
	}
	if timeout, ok := err.(net.Error); ok && timeout.Timeout() {
		t.Fatal("stalled client survived HTTP read-header deadline")
	}
	status, body, _ := readLAN(t, lanClient(t), info.URL, http.MethodGet)
	if status != http.StatusOK || body != "private configuration" {
		t.Fatal("stalled client prevented legitimate transfer")
	}
}

func TestLANRateLimitRecoversWithoutRevokingToken(t *testing.T) {
	server := newLoopbackLAN(t, 0)
	info := startLAN(t, server)
	client := lanClient(t)
	for range 12 {
		readLAN(t, client, info.URL+"-wrong", http.MethodGet)
	}
	time.Sleep(300 * time.Millisecond)
	status, body, _ := readLAN(t, client, info.URL, http.MethodGet)
	if status != http.StatusOK || body != "private configuration" {
		t.Fatal("rate limiting permanently disabled the unconsumed sharing token")
	}
}

func TestLANQueryAndEncodedPathDoNotMatchBearer(t *testing.T) {
	for _, alter := range []func(string) string{
		func(address string) string { return address + "?download=1" },
		func(address string) string {
			parsed, _ := url.Parse(address)
			return parsed.Scheme + "://" + parsed.Host + "/%73/" + parsed.Path[3:]
		},
	} {
		server := newLoopbackLAN(t, 0)
		info := startLAN(t, server)
		status, _, _ := readLAN(t, lanClient(t), alter(info.URL), http.MethodGet)
		if status != http.StatusNotFound || !server.Status().Active {
			t.Fatal("noncanonical bearer path or query was accepted")
		}
	}
}
