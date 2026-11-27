package app

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSubscriptionRefreshRetainsOldNodesWhenPartiallyUnsupported(t *testing.T) {
	body := "socks5://127.0.0.1:1080#one\nsocks5://127.0.0.1:1081#two"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, body) }))
	defer server.Close()
	s, _ := emptySession()
	ctx := context.Background()
	sub, err := s.Subs().AddFromURL(ctx, "partial", server.URL, false)
	if err != nil {
		t.Fatal(err)
	}
	body = "socks5://127.0.0.1:1080#one\nunsupported://127.0.0.1:1081#two"
	if err = s.Subs().Update(ctx, sub.ID); err == nil {
		t.Fatal("partial refresh reported success")
	}
	list, _ := s.Subs().List(ctx)
	if list[0].NodeCount != 2 || list[0].Status != "error" {
		t.Fatalf("last good subscription lost: %+v", list)
	}
}
