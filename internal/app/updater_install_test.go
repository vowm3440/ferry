package app

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallUpdateVerifiesDigestLaunchesInstallerAndBlocksTampering(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	payload := []byte("MZ-verified-ferry-setup-exe")
	sum := sha256.Sum256(payload)
	var srv *httptest.Server
	srv = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, ".exe") {
			_, _ = w.Write(payload)
			return
		}
		manifest, _ := json.Marshal(map[string]any{
			"version": "0.3.0",
			"channel": "stable",
			"url":     srv.URL + "/Ferry-0.3.0-windows-amd64-setup.exe",
			"sha256":  hex.EncodeToString(sum[:]),
			"size":    len(payload),
		})
		sig := ed25519.Sign(priv, manifest)
		w.Header().Set("X-Ferry-Signature", base64.StdEncoding.EncodeToString(sig))
		_, _ = w.Write(manifest)
	}))
	defer srv.Close()

	s, _ := emptySession()
	ctx := context.Background()
	stageDir := filepath.Join(t.TempDir(), "updates")
	s.SetUpdateSource(&UpdateSource{
		ManifestURL: srv.URL + "/manifest.json",
		PublicKey:   pub,
		Client:      srv.Client(),
		StageDir:    stageDir,
	})

	var launchedPath string
	quitCalled := false
	s.SetUpdateInstaller(func(path string) (bool, error) {
		launchedPath = path
		return strings.HasSuffix(strings.ToLower(path), ".exe"), nil
	}, func() {
		quitCalled = true
	})

	// Cannot install before check/stage.
	if err := s.System().InstallUpdate(ctx); err == nil {
		t.Fatal("expected InstallUpdate before ApplyUpdate to fail")
	}

	if _, err := s.System().CheckUpdate(ctx, "stable"); err != nil {
		t.Fatalf("CheckUpdate failed: %v", err)
	}
	if err := s.System().ApplyUpdate(ctx); err != nil {
		t.Fatalf("ApplyUpdate failed: %v", err)
	}
	// Idempotent ApplyUpdate when valid file is already staged.
	if err := s.System().ApplyUpdate(ctx); err != nil {
		t.Fatalf("idempotent ApplyUpdate failed: %v", err)
	}

	expectedFile := filepath.Join(stageDir, "Ferry-0.3.0.exe")
	if _, err := os.Stat(expectedFile); err != nil {
		t.Fatalf("expected .exe staged file %s: %v", expectedFile, err)
	}

	// Tamper with staged file on disk -> InstallUpdate MUST reject and NOT launch!
	if err := os.WriteFile(expectedFile, []byte("tampered-bytes"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := s.System().InstallUpdate(ctx); err == nil {
		t.Fatal("expected tampered staged binary to be rejected")
	}
	if launchedPath != "" || quitCalled {
		t.Fatal("tampered binary must never be launched")
	}

	// Restore valid bytes -> InstallUpdate succeeds, launches .exe, and invokes quit!
	if err := os.WriteFile(expectedFile, payload, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := s.System().InstallUpdate(ctx); err != nil {
		t.Fatalf("InstallUpdate failed on valid binary: %v", err)
	}
	if launchedPath != expectedFile || !quitCalled {
		t.Fatalf("expected installer launched (%q) and quitCalled=%v", launchedPath, quitCalled)
	}
}

func TestCheckUpdateSupportsStaticCompanionSigFile(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte("setup"))
	var manifestBytes []byte
	var sigText string
	var srv *httptest.Server
	srv = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/releases/latest/download/manifest-stable.json":
			_, _ = w.Write(manifestBytes)
		case "/releases/latest/download/manifest-stable.json.sig":
			_, _ = w.Write([]byte(sigText + "\n"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	manifestBytes, _ = json.Marshal(map[string]any{
		"version": "0.2.0",
		"channel": "stable",
		"url":     srv.URL + "/Ferry-0.2.0-windows-amd64-setup.exe",
		"sha256":  hex.EncodeToString(sum[:]),
		"size":    5,
	})
	sigText = base64.StdEncoding.EncodeToString(ed25519.Sign(priv, manifestBytes))

	s, _ := emptySession()
	s.SetUpdateSource(&UpdateSource{
		ManifestURL: srv.URL + "/releases/latest/download/manifest-{channel}.json",
		PublicKey:   pub,
		Client:      srv.Client(),
		StageDir:    filepath.Join(t.TempDir(), "updates"),
	})
	info, err := s.System().CheckUpdate(context.Background(), "stable")
	if err != nil || !info.Available || info.Version != "0.2.0" {
		t.Fatalf("companion .sig CheckUpdate failed: %v %+v", err, info)
	}
}
