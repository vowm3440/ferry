package app

import (
	"compress/gzip"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestSubscriptionGzipLimitAndRedirectDoNotExposeReferer(t *testing.T) {
	var leaked atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			http.Redirect(w, r, "/final", http.StatusFound)
		case "/final":
			leaked.Store(r.Header.Get("Referer") != "")
			fmt.Fprint(w, "socks5://127.0.0.1:1080")
		case "/gzip":
			w.Header().Set("Content-Encoding", "gzip")
			writer := gzip.NewWriter(w)
			fmt.Fprint(writer, strings.Repeat("x", 2<<20))
			writer.Close()
		}
	}))
	defer server.Close()
	if _, _, err := fetchSubscription(context.Background(), server.URL+"/token?secret=test", "clash.meta"); err != nil || leaked.Load() {
		t.Fatal("redirect failed or exposed subscription token in Referer")
	}
	if _, _, err := fetchSubscription(context.Background(), server.URL+"/gzip", "clash.meta"); err == nil {
		t.Fatal("decompression bomb accepted")
	}
}

func TestUpdateDoesNotOverwriteConcurrentEdits(t *testing.T) {
	var updating atomic.Bool
	started, release := make(chan struct{}), make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if updating.Load() {
			close(started)
			select {
			case <-release:
			case <-r.Context().Done():
				return
			}
		}
		fmt.Fprint(w, "socks5://127.0.0.1:1080")
	}))
	defer server.Close()
	session, _ := emptySession()
	subs, ctx := session.Subs(), context.Background()
	sub, err := subs.AddFromURL(ctx, "initial", server.URL, false)
	if err != nil {
		t.Fatal(err)
	}
	updating.Store(true)
	done := make(chan error, 1)
	go func() { done <- subs.Update(ctx, sub.ID) }()
	<-started
	if err := subs.Rename(ctx, sub.ID, "newer-name"); err != nil {
		t.Fatal(err)
	}
	close(release)
	if err := <-done; err == nil {
		t.Fatal("refresh overwrote concurrent edit")
	}
	list, _ := subs.List(ctx)
	if list[0].Name != "newer-name" {
		t.Fatal("concurrent rename lost")
	}
}

func TestAutomaticRefreshAndLiveCoreReload(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		fmt.Fprint(w, "socks5://127.0.0.1:1080")
	}))
	defer server.Close()
	session, remote := emptySession()
	subs, ctx := session.Subs(), context.Background()
	sub, err := subs.AddFromURL(ctx, "auto", server.URL, true)
	if err != nil {
		t.Fatal(err)
	}
	if err := subs.SetAutoUpdate(ctx, sub.ID, true, 1); err != nil {
		t.Fatal(err)
	}
	core, _, _ := session.Services()
	if err := core.Start(ctx); err != nil {
		t.Fatal(err)
	}
	subs.updateDue(ctx, time.Now())
	if calls.Load() != 1 {
		t.Fatal("subscription refreshed before due")
	}
	subs.updateDue(ctx, time.Now().Add(2*time.Hour))
	if calls.Load() != 2 || remote.starts != 2 || core.Status(ctx).Status != "running" {
		t.Fatal("due subscription was not refreshed into the running core")
	}
}
