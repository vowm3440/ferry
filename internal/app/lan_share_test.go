package app

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"ferry/internal/share"
)

func TestLANShareSnapshotExcludesSubscriptionAccessURL(t *testing.T) {
	s, _ := emptySession()
	ctx := context.Background()
	if _, err := s.Subs().AddFromText(ctx, "share", "socks5://user:node-password@127.0.0.1:1080"); err != nil {
		t.Fatal(err)
	}
	s.subs[0].url = "https://subscription.example/private-access-token"
	server, err := share.NewLAN(share.LANOptions{BindIP: "127.0.0.1"})
	if err != nil {
		t.Fatal(err)
	}
	s.lan.server = server
	defer s.CloseShares()
	if s.Subs().LanShareStatus(ctx).Active {
		t.Fatal("server active without user action")
	}
	info, err := s.Subs().LanShareStart(ctx)
	if err != nil || !info.Active {
		t.Fatalf("share start: %v", err)
	}
	response, err := http.Get(info.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	data, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != 200 || !strings.Contains(string(data), "node-password") || strings.Contains(string(data), "private-access-token") {
		t.Fatal("shared wrong credential scope")
	}
	if s.Subs().LanShareStatus(ctx).Active {
		t.Fatal("token still active after one fetch")
	}
	if err := s.Subs().LanShareStop(ctx); err != nil {
		t.Fatal(err)
	}
}
