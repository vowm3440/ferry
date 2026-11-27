//go:build windows

package store

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"
)

func TestLockedDestinationPreservesEncryptedStateAndCleansTemp(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.dat")
	file := New(path)
	if err := file.Save([]byte(`{"previous":true}`)); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	handle, err := windows.CreateFile(name, windows.GENERIC_READ, windows.FILE_SHARE_READ, nil, windows.OPEN_EXISTING, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer windows.CloseHandle(handle)
	if err := file.Save([]byte(`{"replacement":true}`)); err == nil {
		t.Fatal("save succeeded across locked destination")
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("failed rename damaged encrypted state")
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil || len(entries) != 1 {
		t.Fatal("failed replace leaked temporary ciphertext")
	}
}
