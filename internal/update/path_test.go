package update_test

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"ferry/internal/update"
)

func TestDownloadRejectsInvalidDestinationBeforeNetwork(t *testing.T) {
	directory := t.TempDir()
	manifest := trustedManifest(t, "https://example.invalid/artifact")
	paths := []string{"", ".", filepath.Join(directory, "missing", "Ferry.zip"),
		filepath.Join(directory, "Ferry.zip:stream"), directory}
	for _, path := range paths {
		if err := update.Download(context.Background(), manifest, path); err == nil {
			t.Errorf("invalid destination accepted: %q", path)
		}
	}
	assertDirectoryEmpty(t, directory)
}

func TestDownloadPreservesSymlinkDestination(t *testing.T) {
	directory := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.zip")
	if err := os.WriteFile(outside, []byte("keep outside"), 0600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, "Ferry.zip")
	if err := os.Symlink(outside, path); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	err := update.Download(context.Background(), trustedManifest(t, "https://example.invalid/artifact"), path)
	if err == nil {
		t.Fatal("symlink destination accepted")
	}
	data, _ := os.ReadFile(outside)
	if string(data) != "keep outside" {
		t.Fatalf("symlink target changed: %q", data)
	}
}

func TestDownloadPreservesHardLinkDestination(t *testing.T) {
	directory := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.zip")
	if err := os.WriteFile(outside, []byte("keep outside"), 0600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, "Ferry.zip")
	if err := os.Link(outside, path); err != nil {
		t.Fatal(err)
	}
	err := update.Download(context.Background(), trustedManifest(t, "https://example.invalid/artifact"), path)
	if err == nil {
		t.Fatal("hard-link destination accepted")
	}
	data, _ := os.ReadFile(outside)
	if string(data) != "keep outside" {
		t.Fatalf("hard-link target changed: %q", data)
	}
}

func TestDownloadCancelledBeforeStartDoesNotWrite(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	directory := t.TempDir()
	err := update.Download(ctx, trustedManifest(t, "https://example.invalid/artifact"), filepath.Join(directory, "Ferry.zip"))
	if err != context.Canceled {
		t.Fatalf("pre-cancelled download returned %v", err)
	}
	assertDirectoryEmpty(t, directory)
}

func TestDownloadUsesFrozenManifestAfterSourceBytesMutate(t *testing.T) {
	server := artifactServer(t, func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("hello")) })
	raw := manifestJSON("1.2.0", "stable", server.URL, 5)
	m, err := verifyFixture(t, raw, "1.0.0", "stable")
	if err != nil {
		t.Fatal(err)
	}
	clear(raw)
	path := filepath.Join(t.TempDir(), "Ferry.zip")
	if err := update.NewDownloader(server.Client()).Download(context.Background(), m, path); err != nil {
		t.Fatalf("source bytes changed verified artifact: %v", err)
	}
	data, _ := os.ReadFile(path)
	if string(data) != "hello" {
		t.Fatalf("unexpected artifact %q", data)
	}
}
