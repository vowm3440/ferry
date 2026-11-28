//go:build with_clash_api

package host

import (
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
)

func TestSessionDownloadCountsOnlyProxyTrafficNotDirect(t *testing.T) {
	payload := bytes.Repeat([]byte("A"), 16*1024)
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(payload)
	}))
	defer origin.Close()

	upstream := newSOCKS(t)
	s := hostFixture(t, upstream.Addr().(*net.TCPAddr).Port)
	h := New()
	t.Cleanup(func() { _ = h.Stop(context.Background()) })
	if err := h.Start(context.Background(), s); err != nil {
		t.Fatal(err)
	}

	fetch := func() {
		t.Helper()
		proxyURL, _ := url.Parse("http://127.0.0.1:" + strconv.Itoa(int(s.Inbound.MixedPort)))
		transport := &http.Transport{Proxy: http.ProxyURL(proxyURL), DisableKeepAlives: true}
		defer transport.CloseIdleConnections()
		resp, err := (&http.Client{Transport: transport}).Get(origin.URL)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil || len(body) != len(payload) {
			t.Fatalf("unexpected response size %d err=%v", len(body), err)
		}
	}

	// 1. Direct traffic (e.g. domestic/CN direct) must NOT be counted in TotalDown.
	if err := h.SetMode(context.Background(), "direct"); err != nil {
		t.Fatal(err)
	}
	fetch()
	if upstream.requests.Load() != 0 {
		t.Fatalf("direct request unexpectedly hit proxy upstream: %d", upstream.requests.Load())
	}
	directSnap, err := h.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if directSnap.TotalDown != 0 {
		t.Fatalf("direct download must not be counted in TotalDown, got %d", directSnap.TotalDown)
	}

	// 2. Proxy traffic MUST be counted in TotalDown.
	if err := h.SetMode(context.Background(), "global"); err != nil {
		t.Fatal(err)
	}
	fetch()
	if upstream.requests.Load() == 0 {
		t.Fatal("global request did not use proxy upstream")
	}
	proxySnap, err := h.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if proxySnap.TotalDown < int64(len(payload)) {
		t.Fatalf("proxy download not counted in TotalDown: got %d, want >= %d", proxySnap.TotalDown, len(payload))
	}

	// 3. Subsequent direct traffic must not increase TotalDown further.
	if err := h.SetMode(context.Background(), "direct"); err != nil {
		t.Fatal(err)
	}
	fetch()
	afterDirectSnap, err := h.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if afterDirectSnap.TotalDown != proxySnap.TotalDown {
		t.Fatalf("subsequent direct download inflated TotalDown: before=%d after=%d", proxySnap.TotalDown, afterDirectSnap.TotalDown)
	}
}
