//go:build windows

package winsvc

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStageServiceValidationAndCopyAtomic(t *testing.T) {
	if err := Install("S-1-5-18", `C:\missing\ferry-core.exe`, `C:\missing\Ferry.exe`); err == nil {
		t.Fatal("expected SYSTEM sid to be rejected")
	}
	if err := Install("S-1-5-21-1-2-3-1001", `C:\missing\ferry-core.exe`, `C:\missing\Ferry.exe`); err == nil {
		t.Fatal("expected missing GUI exe to be rejected")
	}
	dir := t.TempDir()
	src := filepath.Join(dir, "src.exe")
	dst := filepath.Join(dir, "dst.exe")
	if err := os.WriteFile(src, []byte("synthetic-core-binary"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := copyFileAtomic(src, dst); err != nil {
		t.Fatalf("copyFileAtomic failed: %v", err)
	}
	got, err := os.ReadFile(dst)
	if err != nil || string(got) != "synthetic-core-binary" {
		t.Fatalf("copied content mismatch: %q %v", string(got), err)
	}
	if err := copyFileAtomic("relative.exe", dst); err == nil {
		t.Fatal("relative source path must be rejected")
	}
}
