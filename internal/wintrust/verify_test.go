package wintrust

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVerifyRejectsMissingPinsBeforeFileAccess(t *testing.T) {
	path := filepath.Join(t.TempDir(), "never-created.exe")
	for _, name := range []string{path, "\x00invalid path"} {
		file, err := Verify(name, nil)
		if file != nil || !errors.Is(err, ErrMissingPins) {
			t.Fatalf("empty trust list result = %v, %v", file, err)
		}
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("verification created a file: %v", err)
	}
}

func TestVerifyRejectsMalformedPinsBeforeFileAccess(t *testing.T) {
	file, err := Verify("\x00private-name.exe", []string{strings.Repeat("a", 64), "bad"})
	if file != nil || !errors.Is(err, ErrInvalidPin) {
		t.Fatalf("malformed trust list result = %v, %v", file, err)
	}
	if strings.Contains(err.Error(), "private-name") {
		t.Fatal("file path leaked into error")
	}
}
