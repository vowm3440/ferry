package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"golang.org/x/sys/windows/registry"
)

type memoryStorage struct {
	data []byte
}

func (m *memoryStorage) Load() ([]byte, error) { return append([]byte(nil), m.data...), nil }
func (m *memoryStorage) Save(b []byte) error   { m.data = append([]byte(nil), b...); return nil }

func TestSubCustomUserAgentOverrideAndFallback(t *testing.T) {
	var mu sync.Mutex
	var seenUAs []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		seenUAs = append(seenUAs, r.Header.Get("User-Agent"))
		mu.Unlock()
		_, _ = w.Write([]byte("socks5://127.0.0.1:1080#SubNode\n"))
	}))
	defer srv.Close()

	store := &memoryStorage{}
	s, err := NewPersistentSession(&remoteFixture{}, store)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	sub, err := s.Subs().AddFromURL(ctx, "Airport", srv.URL+"/sub", true)
	if err != nil {
		t.Fatalf("AddFromURL failed: %v", err)
	}
	if err := s.Subs().SetUserAgent(ctx, sub.ID, "FerryCustom/2.0"); err != nil {
		t.Fatalf("SetUserAgent failed: %v", err)
	}
	if err := s.Subs().Update(ctx, sub.ID); err != nil {
		t.Fatalf("Update with custom UA failed: %v", err)
	}

	// Reload from persistent storage and verify custom UA survived.
	reloaded, err := NewPersistentSession(&remoteFixture{}, store)
	if err != nil {
		t.Fatalf("reload failed: %v", err)
	}
	list, _ := reloaded.Subs().List(ctx)
	if len(list) != 1 || list[0].UserAgent != "FerryCustom/2.0" {
		t.Fatalf("expected persisted custom UA, got %+v", list)
	}

	// Clearing custom UA falls back to global Settings.SubscriptionUA.
	if err := reloaded.Subs().SetUserAgent(ctx, sub.ID, ""); err != nil {
		t.Fatalf("clearing custom UA failed: %v", err)
	}
	cfg, _ := (&Settings{reloaded}).Get(ctx)
	cfg.SubscriptionUA = "GlobalUA/3.0"
	if err := (&Settings{reloaded}).Set(ctx, cfg); err != nil {
		t.Fatalf("Settings.Set failed: %v", err)
	}
	if err := reloaded.Subs().Update(ctx, sub.ID); err != nil {
		t.Fatalf("Update with global UA failed: %v", err)
	}
	list, _ = reloaded.Subs().List(ctx)
	if list[0].UserAgent != "GlobalUA/3.0" {
		t.Fatalf("expected global UA fallback, got %q", list[0].UserAgent)
	}

	mu.Lock()
	got := append([]string(nil), seenUAs...)
	mu.Unlock()
	if len(got) != 3 || got[0] != "clash.meta" || got[1] != "FerryCustom/2.0" || got[2] != "GlobalUA/3.0" {
		t.Fatalf("unexpected UA sequence: %v", got)
	}

	// Boundary checks: manual sub, oversized UA, CRLF injection.
	manual, err := reloaded.Subs().AddFromText(ctx, "Manual", "socks5://127.0.0.1:1081#M1")
	if err != nil {
		t.Fatal(err)
	}
	if err := reloaded.Subs().SetUserAgent(ctx, manual.ID, "Custom"); err == nil {
		t.Fatal("expected manual subscription to reject SetUserAgent")
	}
	for _, bad := range []string{strings.Repeat("a", 257), "bad\r\nheader"} {
		if err := reloaded.Subs().SetUserAgent(ctx, sub.ID, bad); err == nil {
			t.Fatalf("expected invalid UA %q to fail", bad)
		}
	}
}

func TestCustomGroupsSaveCycleAndRemove(t *testing.T) {
	s, _ := emptySession()
	ctx := context.Background()
	if _, err := s.Subs().AddFromText(ctx, "Nodes", "socks5://127.0.0.1:1080#N1\nsocks5://127.0.0.1:1081#N2"); err != nil {
		t.Fatal(err)
	}
	nodesSvc := &Nodes{s}
	initial, _ := nodesSvc.List(ctx)
	n1, n2 := initial.Nodes[0].ID, initial.Nodes[1].ID

	// Synthetic default "proxy" group cannot be deleted when no explicit groups exist.
	if err := nodesSvc.RemoveGroup(ctx, "proxy"); err == nil {
		t.Fatal("expected synthetic proxy group deletion to be rejected")
	}

	autoGrp, err := nodesSvc.SaveGroup(ctx, Group{
		Name:        "自动择优",
		Type:        "urltest",
		Members:     []string{n1, n2},
		TestURL:     "https://cp.cloudflare.com/generate_204",
		IntervalMin: 5,
	})
	if err != nil || autoGrp.ID == "" {
		t.Fatalf("create urltest group failed: %v %+v", err, autoGrp)
	}

	mainGrp, err := nodesSvc.SaveGroup(ctx, Group{
		Name:    "主出口",
		Type:    "select",
		Members: []string{autoGrp.ID, n1},
	})
	if err != nil || mainGrp.ID == "" {
		t.Fatalf("create select group failed: %v %+v", err, mainGrp)
	}

	// Cycle detection: autoGrp -> mainGrp -> autoGrp must fail.
	autoGrp.Members = []string{mainGrp.ID}
	if _, err := nodesSvc.SaveGroup(ctx, autoGrp); err == nil {
		t.Fatal("expected circular group reference to be rejected")
	}

	// Deleting autoGrp while mainGrp references it must fail.
	if err := nodesSvc.RemoveGroup(ctx, autoGrp.ID); err == nil {
		t.Fatal("expected referenced group deletion to be rejected")
	}

	// Update mainGrp to drop autoGrp reference, then RemoveGroup succeeds.
	mainGrp.Members = []string{n1, n2}
	if _, err := nodesSvc.SaveGroup(ctx, mainGrp); err != nil {
		t.Fatalf("update mainGrp failed: %v", err)
	}
	if err := nodesSvc.RemoveGroup(ctx, autoGrp.ID); err != nil {
		t.Fatalf("RemoveGroup failed after unreferencing: %v", err)
	}
	after, _ := nodesSvc.List(ctx)
	if len(after.Groups) != 1 || after.Groups[0].ID != mainGrp.ID {
		t.Fatalf("unexpected groups after removal: %+v", after.Groups)
	}
}

func TestRegistryDeepLinkRegisterAndUnregister(t *testing.T) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		t.Fatal(err)
	}
	rootKey := `Software\FerryTestClasses-` + hex.EncodeToString(b[:]) + `\ferry`
	parentKey := filepath.Dir(rootKey)
	exePath := filepath.Join(t.TempDir(), "Ferry.exe")
	reg := &RegistryDeepLink{
		KeyPath:    rootKey,
		Executable: exePath,
	}
	t.Cleanup(func() {
		_ = reg.SetRegistered(false)
		_ = registry.DeleteKey(registry.CURRENT_USER, parentKey)
	})

	s, _ := emptySession()
	s.SetDeepLinkRegistrar(reg)
	ctx := context.Background()

	caps := s.System().Capabilities(ctx)
	hasCap := false
	for _, c := range caps {
		if c == "deeplink.register" {
			hasCap = true
		}
	}
	if !hasCap {
		t.Fatalf("expected deeplink.register capability, got %v", caps)
	}

	on, err := s.System().DeepLinkStatus(ctx)
	if err != nil || on {
		t.Fatalf("expected initial unregistered state, got on=%v err=%v", on, err)
	}
	if err := s.System().SetDeepLinkRegistered(ctx, true); err != nil {
		t.Fatalf("SetDeepLinkRegistered(true) failed: %v", err)
	}
	on, err = s.System().DeepLinkStatus(ctx)
	if err != nil || !on {
		t.Fatalf("expected registered=true, got on=%v err=%v", on, err)
	}
	if err := s.System().SetDeepLinkRegistered(ctx, false); err != nil {
		t.Fatalf("SetDeepLinkRegistered(false) failed: %v", err)
	}
	if err := s.System().SetDeepLinkRegistered(ctx, false); err != nil {
		t.Fatalf("idempotent SetDeepLinkRegistered(false) failed: %v", err)
	}
	on, err = s.System().DeepLinkStatus(ctx)
	if err != nil || on {
		t.Fatalf("expected unregistered after removal, got on=%v err=%v", on, err)
	}
}
