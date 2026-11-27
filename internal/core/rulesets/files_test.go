package rulesets

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUnsafeRootFailsBeforeNetworkWithoutDisclosingPath(t *testing.T) {
	file := filepath.Join(t.TempDir(), "private-root-name")
	writeFixture(t, file, []byte("keep"))
	for _, root := range []string{"", ".", "relative", file, filepath.Join(file, "child")} {
		m := NewWithClient(root, offlineClient(t))
		err := m.Update(context.Background(), "geosite-cn")
		if err == nil || strings.Contains(err.Error(), "private-root-name") {
			t.Fatalf("unsafe root error = %v", err)
		}
	}
	assertFileBytes(t, file, []byte("keep"))
}

func TestCacheRejectsSymlinkRootAndFiles(t *testing.T) {
	for _, directory := range []bool{false, true} {
		root := t.TempDir()
		outside := t.TempDir()
		data := fixtureSRS(t, "outside.example.cn")
		writeFixture(t, filepath.Join(outside, "geosite-cn.srs"), data)
		destination, target := filepath.Join(root, "geosite-cn.srs"), filepath.Join(outside, "geosite-cn.srs")
		if directory {
			destination, target = filepath.Join(root, "link"), outside
		}
		if err := os.Symlink(target, destination); err != nil {
			t.Skipf("symlink unavailable: %v", err)
		}
		if directory {
			root = destination
		}
		m := NewWithClient(root, offlineClient(t))
		if _, err := m.Load("geosite-cn"); err == nil {
			t.Fatal("symlink accepted for reading")
		}
		if err := m.Update(context.Background(), "geosite-cn"); err == nil {
			t.Fatal("symlink accepted for update")
		}
		assertFileBytes(t, filepath.Join(outside, "geosite-cn.srs"), data)
	}
}

func TestDirectoryDestinationRejectedBeforeDownload(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "geosite-cn.srs"), 0700); err != nil {
		t.Fatal(err)
	}
	m := NewWithClient(root, offlineClient(t))
	if err := m.Update(context.Background(), "geosite-cn"); err == nil {
		t.Fatal("directory replaced by download")
	}
	if _, err := m.Load("geosite-cn"); err == nil {
		t.Fatal("directory accepted as cached SRS")
	}
}

func TestConcurrentDestinationChangeIsNotReplaced(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "geosite-cn.srs")
	data := fixtureSRS(t, "new.example.cn")
	client := tlsClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if err := os.Mkdir(path, 0700); err != nil {
			t.Error(err)
		}
		_, _ = w.Write(data)
	}))
	if err := NewWithClient(root, client).Update(context.Background(), "geosite-cn"); err == nil {
		t.Fatal("replaced concurrent directory")
	}
	if info, err := os.Stat(path); err != nil || !info.IsDir() {
		t.Fatalf("destination directory lost: %v", err)
	}
	assertNoStagingFiles(t, root)
}

func TestLocalOversizeCacheRejected(t *testing.T) {
	root := t.TempDir()
	file, err := os.Create(filepath.Join(root, "geosite-cn.srs"))
	if err != nil {
		t.Fatal(err)
	}
	if err = file.Truncate(16<<20 + 1); err != nil {
		t.Fatal(err)
	}
	if err = file.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err = New(root).Load("geosite-cn"); err == nil {
		t.Fatal("oversize local cache accepted")
	}
}
