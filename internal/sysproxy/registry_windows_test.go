//go:build windows

package sysproxy

import (
	"crypto/rand"
	"encoding/hex"
	"ferry/internal/store"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows/registry"
)

func TestWindowsRegistryIsolatedKeyRoundTrip(t *testing.T) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		t.Fatal(err)
	}
	keyPath := `Software\FerryTestSysProxy-` + hex.EncodeToString(b[:])
	t.Cleanup(func() { _ = registry.DeleteKey(registry.CURRENT_USER, keyPath) })

	reg := windowsRegistry{keyPath: keyPath, skipNotify: true}
	empty, err := reg.Read()
	if err != nil || empty.Present != 0 {
		t.Fatalf("expected empty missing key read: %+v %v", empty, err)
	}

	journal := store.New(filepath.Join(t.TempDir(), "proxy-restore.dat"))
	mgr := New(reg, journal)
	if err := mgr.Enable(7890); err != nil {
		t.Fatalf("Enable failed: %v", err)
	}
	on, addr, err := mgr.Status()
	if err != nil || !on || addr != "127.0.0.1:7890" {
		t.Fatalf("unexpected enabled status: on=%v addr=%q err=%v", on, addr, err)
	}
	if err := mgr.Disable(); err != nil {
		t.Fatalf("Disable failed: %v", err)
	}
	restored, err := reg.Read()
	if err != nil || restored.Present != 0 {
		t.Fatalf("expected clean restore removing created values: %+v %v", restored, err)
	}
	// Pre-existing values including PAC should be read and restored faithfully.
	existing := Config{Present: 1 | 2 | 4 | 8, Enabled: 1, Server: "10.0.0.1:8080", Bypass: "localhost", PAC: "http://pac.local/p.pac"}
	if err := reg.Write(existing); err != nil {
		t.Fatalf("pre-seed Write failed: %v", err)
	}
	got, err := reg.Read()
	if err != nil || got != existing {
		t.Fatalf("Read mismatch: got=%+v want=%+v err=%v", got, existing, err)
	}
	if err := mgr.Enable(7890); err != nil {
		t.Fatalf("Enable over existing failed: %v", err)
	}
	if err := mgr.Disable(); err != nil {
		t.Fatalf("Disable restore existing failed: %v", err)
	}
	after, err := reg.Read()
	if err != nil || after != existing {
		t.Fatalf("expected pre-existing config restored: got=%+v want=%+v err=%v", after, existing, err)
	}
	if NewWindows(journal) == nil {
		t.Fatal("NewWindows returned nil")
	}
}
