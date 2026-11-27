package app

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestURLSubscriptionMetadataUpdateAndRedaction(t *testing.T) {
	var generation atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") != "clash.meta" {
			t.Error("default subscription user agent missing")
		}
		w.Header().Set("subscription-userinfo", "upload=10; download=20; total=1000; expire=2000000000")
		w.Header().Set("profile-update-interval", "12")
		w.Header().Set("profile-title", "base64:VGVzdCBQcm9maWxl")
		fmt.Fprintf(w, "socks5://127.0.0.1:%d#node", 1080+generation.Load())
	}))
	defer server.Close()
	session, _ := emptySession()
	ctx := context.Background()
	subs := session.Subs()
	sub, err := subs.AddFromURL(ctx, "", server.URL+"/private-token?auth=synthetic", true)
	if err != nil {
		t.Fatal(err)
	}
	if sub.Name != "Test Profile" || sub.UsedBytes != 30 || sub.TotalBytes != 1000 || sub.ExpiresAt != 2000000000000 || sub.IntervalHours != 12 || !sub.AutoUpdate {
		t.Fatalf("incorrect metadata: %+v", sub)
	}
	if strings.Contains(sub.URLMasked, "private-token") || strings.Contains(sub.URLMasked, "synthetic") || len(sub.Warnings) == 0 {
		t.Fatal("URL was exposed or HTTP warning missing")
	}
	if err := subs.Rename(ctx, sub.ID, "renamed"); err != nil {
		t.Fatal(err)
	}
	generation.Store(1)
	if err := subs.Update(ctx, sub.ID); err != nil {
		t.Fatal(err)
	}
	list, _ := (&Nodes{session}).List(ctx)
	listed, _ := subs.List(ctx)
	if len(list.Nodes) != 1 || list.Nodes[0].Port != 1081 || listed[0].ID != sub.ID || listed[0].Name != "renamed" {
		t.Fatal("update did not replace owned nodes while preserving identity and name")
	}
}

func TestFailedURLUpdateKeepsPreviousNodes(t *testing.T) {
	var fail atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if fail.Load() {
			http.Error(w, "synthetic-token", http.StatusBadGateway)
			return
		}
		fmt.Fprint(w, "socks5://127.0.0.1:1080")
	}))
	defer server.Close()
	session, _ := emptySession()
	subs, ctx := session.Subs(), context.Background()
	sub, err := subs.AddFromURL(ctx, "keep", server.URL, false)
	if err != nil {
		t.Fatal(err)
	}
	fail.Store(true)
	if err := subs.Update(ctx, sub.ID); err == nil || strings.Contains(err.Error(), "synthetic-token") {
		t.Fatalf("failed update error: %v", err)
	}
	list, _ := (&Nodes{session}).List(ctx)
	if len(list.Nodes) != 1 || list.Nodes[0].Port != 1080 {
		t.Fatal("failed update removed usable nodes")
	}
}

func TestURLImportRejectsUnsafeOrOversizedInput(t *testing.T) {
	session, _ := emptySession()
	subs, ctx := session.Subs(), context.Background()
	for _, raw := range []string{"file:///secret", "https://user:password@example.com", "ftp://example.com", "https://example.com/#secret"} {
		if _, err := subs.AddFromURL(ctx, "", raw, false); err == nil {
			t.Fatal("unsafe URL accepted")
		}
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, strings.Repeat("x", (1<<20)+1))
	}))
	defer server.Close()
	if _, err := subs.AddFromURL(ctx, "", server.URL, false); err == nil {
		t.Fatal("oversized response accepted")
	}
	if len(session.subs) != 0 {
		t.Fatal("failed imports changed subscriptions")
	}
}
