package sysproxy

import (
	"encoding/json"
	"testing"
)

func TestRecoveryHandlesInterruptedMultiValueWrite(t *testing.T) {
	original := Config{Enabled: 0, Server: "old:1080", Bypass: "original", PAC: "https://local/pac", Present: 15}
	applied := Config{Enabled: 1, Server: "127.0.0.1:17890", Bypass: "<local>", Present: 7}
	partial := original
	partial.Enabled = 1
	partial.Server = applied.Server
	registry := &memoryRegistry{value: partial}
	journal := &memoryJournal{}
	journal.data, _ = json.Marshal(record{Version: 1, Active: true, Original: original, Applied: applied})
	if err := New(registry, journal).Recover(); err != nil {
		t.Fatal(err)
	}
	if registry.value != original {
		t.Fatalf("partial write left dead proxy: %+v", registry.value)
	}
}

func TestRecoveryPreservesForeignChangeToSingleField(t *testing.T) {
	original := Config{Enabled: 0, Server: "old:1080", Present: 3}
	registry := &memoryRegistry{value: original}
	journal := &memoryJournal{}
	manager := New(registry, journal)
	if err := manager.Enable(17890); err != nil {
		t.Fatal(err)
	}
	registry.value.PAC = "https://new-client/pac"
	registry.value.Present |= 8
	if err := manager.Disable(); err != nil {
		t.Fatal(err)
	}
	if registry.value.PAC != "https://new-client/pac" || registry.value.Server != "old:1080" || registry.value.Enabled != 0 {
		t.Fatalf("foreign PAC or own proxy cleanup lost: %+v", registry.value)
	}
}

func TestRecoveryNeverDisablesForeignProxyServer(t *testing.T) {
	original := Config{Enabled: 0, Server: "", Present: 3}
	registry := &memoryRegistry{value: original}
	journal := &memoryJournal{}
	manager := New(registry, journal)
	if err := manager.Enable(17890); err != nil {
		t.Fatal(err)
	}
	registry.value.Server = "other-client:8080"
	if err := manager.Disable(); err != nil {
		t.Fatal(err)
	}
	if registry.value.Enabled != 1 || registry.value.Server != "other-client:8080" {
		t.Fatal("another client's proxy was disabled")
	}
}
