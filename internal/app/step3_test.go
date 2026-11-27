package app

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"ferry/internal/release"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/windows/registry"
)

type step3FakeProxy struct{ enabled bool }

func (p *step3FakeProxy) Enable(int) error               { p.enabled = true; return nil }
func (p *step3FakeProxy) Disable() error                 { p.enabled = false; return nil }
func (p *step3FakeProxy) Status() (bool, string, error)  { return p.enabled, "127.0.0.1:7890", nil }

type fakeAutoStart struct {
	enabled bool
	failSet bool
}

func (f *fakeAutoStart) Set(on bool) error {
	if f.failSet {
		return errors.New("autoStart failure")
	}
	f.enabled = on
	return nil
}
func (f *fakeAutoStart) Enabled() (bool, error) { return f.enabled, nil }

func TestScreenQRCaptureImportsNodesAndRejectsRawSubscriptionURL(t *testing.T) {
	s, _ := emptySession()
	ctx := context.Background()
	s.SetScreenCapturer(func() (string, error) {
		return "socks5://127.0.0.1:1080#ScreenNode", nil
	})
	subs, err := s.Subs().CaptureScreenQR(ctx)
	if err != nil || len(subs) != 1 || subs[0].NodeCount != 1 {
		t.Fatalf("screen QR node import failed: %v %+v", err, subs)
	}
	s.SetScreenCapturer(func() (string, error) {
		return "https://sub.example.com/profile", nil
	})
	if _, err := s.Subs().CaptureScreenQR(ctx); err == nil || !strings.Contains(err.Error(), "确认来源") {
		t.Fatalf("expected URL confirmation error, got: %v", err)
	}
}

func TestDeepLinkShareAndParsePreview(t *testing.T) {
	s, _ := emptySession()
	ctx := context.Background()
	links, err := s.Subs().ShareDeepLink(ctx, "https://sub.example.com/list?token=abc")
	if err != nil || len(links) != 3 {
		t.Fatalf("expected 3 client deep links, got %v %+v", err, links)
	}
	preview, err := s.System().ParseDeepLink(ctx, links[0].URL)
	if err != nil || preview.Kind != "url" || preview.Host != "sub.example.com" || preview.Warning != "" {
		t.Fatalf("unexpected HTTPS deeplink preview: %v %+v", err, preview)
	}
	httpPreview, err := s.System().ParseDeepLink(ctx, "clash://install-config?url=http%3A%2F%2Fplain.example.com%2Fsub&name=Test")
	if err != nil || httpPreview.Warning == "" || httpPreview.Name != "Test" {
		t.Fatalf("expected HTTP deeplink warning: %v %+v", err, httpPreview)
	}
	inlineLinks, err := s.Subs().ShareDeepLink(ctx, "socks5://127.0.0.1:1080#n1")
	if err != nil || len(inlineLinks) != 1 {
		t.Fatalf("expected 1 inline deeplink: %v %+v", err, inlineLinks)
	}
	inlinePreview, err := s.System().ParseDeepLink(ctx, inlineLinks[0].URL)
	if err != nil || inlinePreview.Kind != "text" || inlinePreview.Host != "127.0.0.1" {
		t.Fatalf("unexpected inline deeplink preview: %v %+v", err, inlinePreview)
	}
	for _, bad := range []string{"", "javascript://alert(1)", "ferry://unknown?url=https://a.com", "ferry://install-config?url=file:///etc/passwd"} {
		if _, err := s.System().ParseDeepLink(ctx, bad); err == nil {
			t.Fatalf("expected invalid deeplink to fail: %q", bad)
		}
	}
}

func TestAutomationSettingsAndSystemTools(t *testing.T) {
	s, _ := configured(t)
	ctx := context.Background()
	auto := &fakeAutoStart{}
	s.SetAutoStartController(auto)
	var opened string
	dir := t.TempDir()
	s.SetDataDirs(dir, filepath.Join(dir, "logs"), func(p string) error { opened = p; return nil })
	var lw bool
	s.SetLightweightHook(func(on bool) error { lw = on; return nil })

	settings := &Settings{s}
	v, _ := settings.Get(ctx)
	v.AutoStart, v.StartMinimized, v.Lightweight, v.SysProxyOnStart, v.RuleSetAutoUpdate = true, true, true, true, true
	v.Language = "en-US"
	if err := settings.Set(ctx, v); err != nil {
		t.Fatalf("expected automation settings to succeed: %v", err)
	}
	if !auto.enabled {
		t.Fatal("AutoStart controller was not invoked")
	}
	auto.failSet = true
	v.AutoStart = false
	if err := settings.Set(ctx, v); err == nil {
		t.Fatal("expected AutoStart failure to reject settings save")
	}
	if err := s.Window().SetLightweight(ctx, false); err != nil || lw {
		t.Fatalf("Window.SetLightweight failed: %v lw=%v", err, lw)
	}
	if err := s.System().OpenConfigDir(ctx); err != nil || opened != filepath.Clean(dir) {
		t.Fatalf("OpenConfigDir failed: %v opened=%q", err, opened)
	}
	if err := s.System().OpenLogDir(ctx); err != nil || opened != filepath.Join(dir, "logs") {
		t.Fatalf("OpenLogDir failed: %v opened=%q", err, opened)
	}
	if ms, err := s.System().TestInbound(ctx); err != nil || ms < 0 {
		t.Fatalf("TestInbound failed: %v ms=%d", err, ms)
	}
	// Verify SysProxyOnStart enables proxy when core starts
	proxy := &step3FakeProxy{}
	s.SetProxyController(proxy)
	if err := (&Core{s}).Start(ctx); err != nil {
		t.Fatalf("core start failed: %v", err)
	}
	if !proxy.enabled {
		t.Fatal("expected SysProxyOnStart to enable system proxy on core start")
	}
}

func TestUpdaterUnconfiguredAndSignedDownloadFlow(t *testing.T) {
	s, _ := emptySession()
	s.SetUpdateSource(&UpdateSource{})
	ctx := context.Background()
	if _, err := s.System().CheckUpdate(ctx, "stable"); !errors.Is(err, release.ErrUpdateUnconfigured) {
		t.Fatalf("expected ErrUpdateUnconfigured, got %v", err)
	}
	st, err := s.Update().GetState(ctx)
	if err != nil || st.Phase != "unconfigured" {
		t.Fatalf("expected unconfigured update state, got %+v %v", st, err)
	}

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	payload := []byte("verified-ferry-release-binary")
	sum := sha256.Sum256(payload)
	var srv *httptest.Server
	srv = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/artifact.bin" {
			_, _ = w.Write(payload)
			return
		}
		manifest, _ := json.Marshal(map[string]any{
			"version": "0.2.0",
			"channel": "stable",
			"url":     srv.URL + "/artifact.bin",
			"sha256":  hex.EncodeToString(sum[:]),
			"size":    len(payload),
		})
		sig := ed25519.Sign(priv, manifest)
		w.Header().Set("X-Ferry-Signature", base64.StdEncoding.EncodeToString(sig))
		_, _ = w.Write(manifest)
	}))
	defer srv.Close()

	stageDir := filepath.Join(t.TempDir(), "updates")
	s.SetUpdateSource(&UpdateSource{
		ManifestURL: srv.URL + "/manifest.json",
		PublicKey:   pub,
		Client:      srv.Client(),
		StageDir:    stageDir,
	})
	info, err := s.System().CheckUpdate(ctx, "stable")
	if err != nil || !info.Available || info.Version != "0.2.0" {
		t.Fatalf("signed CheckUpdate failed: %v %+v", err, info)
	}
	if err := s.System().ApplyUpdate(ctx); err != nil {
		t.Fatalf("signed ApplyUpdate failed: %v", err)
	}
	st, _ = s.Update().GetState(ctx)
	if st.Phase != "staged" || st.Progress != 100 {
		t.Fatalf("expected staged state, got %+v", st)
	}
	stagedBytes, err := os.ReadFile(filepath.Join(stageDir, "Ferry-0.2.0.bin"))
	if err != nil || string(stagedBytes) != string(payload) {
		t.Fatalf("staged artifact mismatch: %v", err)
	}
}

func TestServiceAndTunStatusAndIsolatedRegistryAutoStart(t *testing.T) {
	s, _ := emptySession()
	ctx := context.Background()
	st, err := s.System().ServiceStatus(ctx)
	if err != nil || st.Installed {
		t.Fatalf("unexpected ServiceStatus: %+v %v", st, err)
	}
	tun, err := s.System().TunStatus(ctx)
	if err != nil || tun.Enabled || tun.Running {
		t.Fatalf("unexpected TunStatus: %+v %v", tun, err)
	}
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		t.Fatal(err)
	}
	auto := &RegistryAutoStart{
		KeyPath:    `Software\FerryTestAutoStart-` + hex.EncodeToString(b[:]),
		ValueName:  "Ferry",
		Executable: filepath.Join(t.TempDir(), "Ferry.exe"),
	}
	t.Cleanup(func() { _ = registry.DeleteKey(registry.CURRENT_USER, auto.KeyPath) })
	if on, err := auto.Enabled(); err != nil || on {
		t.Fatalf("expected disabled initial state: %v %v", on, err)
	}
	if err := auto.Set(true); err != nil {
		t.Fatalf("RegistryAutoStart.Set(true) failed: %v", err)
	}
	if on, err := auto.Enabled(); err != nil || !on {
		t.Fatalf("expected enabled state: %v %v", on, err)
	}
	if err := auto.Set(false); err != nil {
		t.Fatalf("RegistryAutoStart.Set(false) failed: %v", err)
	}
	if err := auto.Set(false); err != nil {
		t.Fatalf("idempotent Set(false) failed: %v", err)
	}
	if on, err := auto.Enabled(); err != nil || on {
		t.Fatalf("expected disabled after Set(false): %v %v", on, err)
	}
	badAuto := NewWindowsAutoStart("relative.exe")
	if err := badAuto.Set(true); err == nil {
		t.Fatal("expected relative executable path to fail")
	}
}
