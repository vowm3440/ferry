package rulesets

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestUpdatePublishesValidatedCacheAndSupportsOfflineRestart(t *testing.T) {
	data := fixtureSRS(t, "new.example.cn")
	client := tlsClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/SagerNet/sing-geosite/rule-set/geosite-cn.srs" {
			t.Errorf("unexpected fixed upstream path %q", r.URL.Path)
		}
		_, _ = w.Write(data)
	}))
	root := filepath.Join(t.TempDir(), "rulesets")
	if err := NewWithClient(root, client).Update(context.Background(), "geosite-cn"); err != nil {
		t.Fatal(err)
	}
	m := NewWithClient(root, offlineClient(t))
	path, err := m.Resolve("geosite-cn")
	if err != nil {
		t.Fatal(err)
	}
	assertFileBytes(t, path, data)
	if info := m.List()[0]; info.Status != "ready" || info.SizeBytes != int64(len(data)) {
		t.Fatalf("unexpected metadata: %+v", info)
	}
	assertNoStagingFiles(t, root)
}

func TestFailedDownloadsPreservePreviousCache(t *testing.T) {
	valid := fixtureSRS(t, "old.example.cn")
	cases := map[string]http.HandlerFunc{
		"status":    func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(503) },
		"invalid":   func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("not SRS")) },
		"truncated": func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(valid[:len(valid)-1]) },
		"encoding": func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Encoding", "gzip")
			_, _ = w.Write(valid)
		},
		"length": func(w http.ResponseWriter, _ *http.Request) { w.Header().Set("Content-Length", "16777217") },
		"stream": func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(200)
			w.(http.Flusher).Flush()
			_, _ = io.CopyN(w, zeroReader{}, 16<<20+1)
		},
		"disconnect": func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Length", "1000")
			_, _ = w.Write(valid)
		},
	}
	for name, handler := range cases {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "geosite-cn.srs")
			writeFixture(t, path, valid)
			m := NewWithClient(root, tlsClient(t, handler))
			if err := m.Update(context.Background(), "geosite-cn"); err == nil {
				t.Fatal("bad download accepted")
			}
			assertFileBytes(t, path, valid)
			if _, err := m.Resolve("geosite-cn"); err != nil {
				t.Fatalf("previous cache no longer usable: %v", err)
			}
			if info := m.List()[0]; info.Status != "error" || info.SizeBytes != int64(len(valid)) {
				t.Fatalf("failed refresh lost cache metadata: %+v", info)
			}
			assertNoStagingFiles(t, root)
		})
	}
}

func TestUpdateCancelsActiveAndQueuedRequests(t *testing.T) {
	started := make(chan struct{})
	client := tlsClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-r.Context().Done()
	}))
	root := t.TempDir()
	data := fixtureSRS(t, "old.example.cn")
	path := filepath.Join(root, "geosite-cn.srs")
	writeFixture(t, path, data)
	m := NewWithClient(root, client)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- m.Update(ctx, "geosite-cn") }()
	<-started
	if info := m.List()[0]; info.Status != "updating" {
		t.Fatalf("update not visible: %+v", info)
	}
	queued, cancelQueued := context.WithCancel(context.Background())
	cancelQueued()
	if err := m.Update(queued, "geoip-cn"); !errors.Is(err, context.Canceled) {
		t.Fatalf("queued cancellation = %v", err)
	}
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("active cancellation = %v", err)
	}
	assertFileBytes(t, path, data)
}

func TestUpdateEnforcesClientTimeout(t *testing.T) {
	client := tlsClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	client.Timeout = 20 * time.Millisecond
	err := NewWithClient(t.TempDir(), client).Update(context.Background(), "geosite-cn")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("timeout = %v", err)
	}
}

func TestRedirectsCannotEscapeFixedHTTPSHost(t *testing.T) {
	for _, target := range []string{"http://raw.githubusercontent.com/a", "https://evil.invalid/a", "https://raw.githubusercontent.com:444/a", "https://user@raw.githubusercontent.com/a"} {
		t.Run(target, func(t *testing.T) {
			client := tlsClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, target, http.StatusFound)
			}))
			if err := NewWithClient(t.TempDir(), client).Update(context.Background(), "geosite-cn"); err == nil {
				t.Fatal("unsafe redirect accepted")
			}
		})
	}
}

func TestSafeRedirectHasBoundedHops(t *testing.T) {
	data := fixtureSRS(t, "example.cn")
	for _, loop := range []bool{false, true} {
		client := tlsClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if loop || r.URL.Path != "/target.srs" {
				http.Redirect(w, r, "https://raw.githubusercontent.com/target.srs", http.StatusFound)
				return
			}
			_, _ = w.Write(data)
		}))
		err := NewWithClient(t.TempDir(), client).Update(context.Background(), "geosite-cn")
		if (err != nil) != loop {
			t.Fatalf("loop=%v error=%v", loop, err)
		}
	}
}

func tlsClient(t *testing.T, handler http.Handler) *http.Client {
	t.Helper()
	server := httptest.NewTLSServer(handler)
	t.Cleanup(server.Close)
	target, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	transport := server.Client().Transport
	return &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host != "raw.githubusercontent.com" || r.URL.Scheme != "https" {
			t.Errorf("unapproved network request: %s", r.URL)
			return nil, errors.New("unapproved network request")
		}
		copy := r.Clone(r.Context())
		copy.URL.Host = target.Host
		copy.Host = target.Host
		return transport.RoundTrip(copy)
	})}
}

func assertFileBytes(t *testing.T, path string, expected []byte) {
	t.Helper()
	actual, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(actual, expected) {
		t.Fatalf("cache content changed: length=%d, error=%v", len(actual), err)
	}
}

func assertNoStagingFiles(t *testing.T, root string) {
	t.Helper()
	files, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".tmp") {
			t.Fatalf("staging file remains: %s", file.Name())
		}
	}
}

type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) { clear(p); return len(p), nil }
