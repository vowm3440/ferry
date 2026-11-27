package update_test

import (
	"context"
	"errors"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ferry/internal/update"
)

func trustedManifest(t *testing.T, url string) update.Manifest {
	t.Helper()
	m, err := verifyFixture(t, manifestJSON("1.2.0", "stable", url, 5), "1.0.0", "stable")
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func artifactServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	server := httptest.NewTLSServer(handler)
	t.Cleanup(server.Close)
	return server
}

func assertDirectoryEmpty(t *testing.T, directory string) {
	t.Helper()
	entries, err := os.ReadDir(directory)
	if err != nil || len(entries) != 0 {
		t.Fatalf("download left files after failure: %v (%v)", entries, err)
	}
}

func TestDownloadStagesOnlyVerifiedCompleteArtifact(t *testing.T) {
	server := artifactServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.Header.Get("Accept-Encoding") != "identity" {
			t.Error("download did not request original artifact bytes")
		}
		_, _ = w.Write([]byte("hello"))
	})
	directory := t.TempDir()
	path := filepath.Join(directory, "Ferry.zip")
	err := update.NewDownloader(server.Client()).Download(context.Background(), trustedManifest(t, server.URL), path)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "hello" {
		t.Fatalf("published artifact = %q: %v", data, err)
	}
	entries, _ := os.ReadDir(directory)
	if len(entries) != 1 {
		t.Fatalf("temporary files remain: %v", entries)
	}
}

func TestDownloadRejectsUnverifiedManifestBeforeFilesystemAccess(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "Ferry.zip")
	if err := update.Download(context.Background(), update.Manifest{}, path); err == nil {
		t.Fatal("zero-value manifest authorized download")
	}
	assertDirectoryEmpty(t, directory)
}

func TestDownloadPreservesExistingFile(t *testing.T) {
	server := artifactServer(t, func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("hello")) })
	directory := t.TempDir()
	path := filepath.Join(directory, "Ferry.zip")
	if err := os.WriteFile(path, []byte("keep existing"), 0600); err != nil {
		t.Fatal(err)
	}
	err := update.NewDownloader(server.Client()).Download(context.Background(), trustedManifest(t, server.URL), path)
	if err == nil {
		t.Fatal("existing target was replaced")
	}
	data, _ := os.ReadFile(path)
	if string(data) != "keep existing" {
		t.Fatalf("existing data changed: %q", data)
	}
	entries, _ := os.ReadDir(directory)
	if len(entries) != 1 {
		t.Fatalf("temporary files remain: %v", entries)
	}
}

func TestDownloadDoesNotOverwriteFileCreatedDuringDownload(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "Ferry.zip")
	server := artifactServer(t, func(w http.ResponseWriter, r *http.Request) {
		if err := os.WriteFile(path, []byte("concurrent writer"), 0600); err != nil {
			t.Error(err)
		}
		_, _ = w.Write([]byte("hello"))
	})
	err := update.NewDownloader(server.Client()).Download(context.Background(), trustedManifest(t, server.URL), path)
	if err == nil {
		t.Fatal("download overwrote a concurrently created destination")
	}
	data, _ := os.ReadFile(path)
	if string(data) != "concurrent writer" {
		t.Fatalf("concurrent data changed: %q", data)
	}
	entries, _ := os.ReadDir(directory)
	if len(entries) != 1 {
		t.Fatalf("temporary files remain: %v", entries)
	}
}

func TestDownloadCleansFailedResponses(t *testing.T) {
	cases := []struct {
		name    string
		handler http.HandlerFunc
	}{
		{"digest", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("HELLO")) }},
		{"truncated", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Length", "5")
			_, _ = w.Write([]byte("hell"))
		}},
		{"oversized", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("hello!")) }},
		{"chunked-oversized", func(w http.ResponseWriter, r *http.Request) {
			w.(http.Flusher).Flush()
			_, _ = w.Write([]byte("hello!"))
		}},
		{"chunked-truncated", func(w http.ResponseWriter, r *http.Request) { w.(http.Flusher).Flush(); _, _ = w.Write([]byte("hell")) }},
		{"status", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte("hello"))
		}},
		{"partial", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusPartialContent)
			_, _ = w.Write([]byte("hello"))
		}},
		{"encoded", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Encoding", "gzip")
			_, _ = w.Write([]byte("hello"))
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := artifactServer(t, tc.handler)
			directory := t.TempDir()
			err := update.NewDownloader(server.Client()).Download(context.Background(), trustedManifest(t, server.URL), filepath.Join(directory, "Ferry.zip"))
			if err == nil {
				t.Fatal("invalid response was published")
			}
			assertDirectoryEmpty(t, directory)
		})
	}
}

func TestDownloadCancellationCleansPartialFile(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	server := artifactServer(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("he"))
		w.(http.Flusher).Flush()
		cancel()
		<-r.Context().Done()
	})
	directory := t.TempDir()
	err := update.NewDownloader(server.Client()).Download(ctx, trustedManifest(t, server.URL), filepath.Join(directory, "Ferry.zip"))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("download cancellation = %v", err)
	}
	assertDirectoryEmpty(t, directory)
}

func TestDownloadRejectsUntrustedTLSCertificate(t *testing.T) {
	server := artifactServer(t, func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("hello")) })
	server.Config.ErrorLog = log.New(io.Discard, "", 0) // Expected rejection, not a server fault.
	directory := t.TempDir()
	err := update.Download(context.Background(), trustedManifest(t, server.URL+"/secret-path?token=secret-token"), filepath.Join(directory, "Ferry.zip"))
	if err == nil || strings.Contains(err.Error(), "secret-") {
		t.Fatalf("untrusted TLS result leaked URL or succeeded: %v", err)
	}
	assertDirectoryEmpty(t, directory)
}

func TestDownloadStopsReadingImmediatelyAfterSizeLimit(t *testing.T) {
	server := artifactServer(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("hello!"))
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	})
	client := server.Client()
	client.Timeout = time.Second
	directory := t.TempDir()
	err := update.NewDownloader(client).Download(context.Background(), trustedManifest(t, server.URL), filepath.Join(directory, "Ferry.zip"))
	if err == nil || errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("oversized streaming response was not bounded before timeout: %v", err)
	}
	assertDirectoryEmpty(t, directory)
}

func TestDownloadTimeoutCleansPartialFile(t *testing.T) {
	server := artifactServer(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("he"))
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	})
	client := server.Client()
	client.Timeout = 100 * time.Millisecond
	directory := t.TempDir()
	err := update.NewDownloader(client).Download(context.Background(), trustedManifest(t, server.URL), filepath.Join(directory, "Ferry.zip"))
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("download timeout = %v", err)
	}
	assertDirectoryEmpty(t, directory)
}
