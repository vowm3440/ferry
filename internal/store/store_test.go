package store

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestEncryptedStoreRoundTripAndMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "state.dat")
	file := New(path)
	if data, err := file.Load(); err != nil || data != nil {
		t.Fatalf("missing store: %v", err)
	}
	plain := []byte(`{"password":"synthetic-password","subscription":"https://example.com/token"}`)
	if err := file.Save(plain); err != nil {
		t.Fatal(err)
	}
	cipher, err := os.ReadFile(path)
	if err != nil || bytes.Contains(cipher, []byte("synthetic-password")) || bytes.Contains(cipher, []byte("example.com")) {
		t.Fatal("sensitive state was written in plaintext")
	}
	loaded, err := New(path).Load()
	if err != nil || !bytes.Equal(loaded, plain) {
		t.Fatalf("encrypted state did not reload: %v", err)
	}
}

func TestRejectedWritePreservesPreviousState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.dat")
	file := New(path)
	if err := file.Save([]byte(`{"previous":true}`)); err != nil {
		t.Fatal(err)
	}
	for _, invalid := range [][]byte{nil, []byte("not-json"), bytes.Repeat([]byte("x"), MaxBytes+1)} {
		if err := file.Save(invalid); err == nil {
			t.Fatal("invalid state accepted")
		}
	}
	loaded, err := file.Load()
	if err != nil || string(loaded) != `{"previous":true}` {
		t.Fatal("rejected write damaged previous state")
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil || len(entries) != 1 {
		t.Fatal("temporary save files leaked")
	}
}

func TestCorruptCiphertextRejectedWithoutOverwrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.dat")
	file := New(path)
	if err := file.Save([]byte(`{"secret":"synthetic"}`)); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	data[len(data)-1] ^= 0xff
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := file.Load(); err == nil {
		t.Fatal("tampered ciphertext accepted")
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(after, data) {
		t.Fatal("failed read silently overwrote corrupt data")
	}
}
