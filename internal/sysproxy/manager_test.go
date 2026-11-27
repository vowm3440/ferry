package sysproxy

import (
	"bytes"
	"errors"
	"testing"
)

type memoryRegistry struct {
	value  Config
	writes int
}

func (r *memoryRegistry) Read() (Config, error) { return r.value, nil }
func (r *memoryRegistry) Write(v Config) error  { r.value = v; r.writes++; return nil }

type memoryJournal struct {
	data []byte
	fail bool
}

func (j *memoryJournal) Load() ([]byte, error) { return bytes.Clone(j.data), nil }
func (j *memoryJournal) Save(v []byte) error {
	if j.fail {
		return errors.New("disk failure")
	}
	j.data = bytes.Clone(v)
	return nil
}

func TestProxyRestoresOnlyItsOwnChanges(t *testing.T) {
	original := Config{Enabled: 1, Server: "other-client:7890", Bypass: "example.com", PAC: "https://example.com/proxy.pac"}
	registry := &memoryRegistry{value: original}
	journal := &memoryJournal{}
	manager := New(registry, journal)
	if err := manager.Enable(17890); err != nil {
		t.Fatal(err)
	}
	if registry.value.Server != "127.0.0.1:17890" || registry.value.PAC != "" {
		t.Fatal("manual proxy not applied")
	}
	if err := manager.Disable(); err != nil || registry.value != original {
		t.Fatal("original settings were not restored")
	}
	if err := manager.Enable(17890); err != nil {
		t.Fatal(err)
	}
	registry.value.Server = "new-client:7891"
	if err := manager.Disable(); err != nil || registry.value.Server != "new-client:7891" {
		t.Fatal("overwrote another client's later change")
	}
}

func TestProxyRecoversAfterCrashAndRefusesUnjournaledChange(t *testing.T) {
	original := Config{Server: "prior", Enabled: 0}
	registry := &memoryRegistry{value: original}
	journal := &memoryJournal{}
	if err := New(registry, journal).Enable(17890); err != nil {
		t.Fatal(err)
	}
	if err := New(registry, journal).Recover(); err != nil || registry.value != original {
		t.Fatal("crash recovery failed")
	}
	journal.fail = true
	before := registry.writes
	if err := New(registry, journal).Enable(17890); err == nil || registry.writes != before {
		t.Fatal("proxy changed without durable restoration data")
	}
}
