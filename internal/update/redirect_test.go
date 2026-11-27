package update_test

import (
	"context"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"ferry/internal/update"
)

func TestDownloadRedirectDoesNotLeakCredentialsOrReferer(t *testing.T) {
	var requests atomic.Int32
	server := artifactServer(t, func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		for _, header := range []string{"Authorization", "Proxy-Authorization", "Cookie", "Referer"} {
			if r.Header.Get(header) != "" {
				t.Errorf("request leaked %s", header)
			}
		}
		if r.URL.Path != "/artifact" {
			http.Redirect(w, r, "/artifact", http.StatusFound)
			return
		}
		_, _ = w.Write([]byte("hello"))
	})
	client := server.Client()
	client.Jar, _ = cookiejar.New(nil)
	serverURL, _ := url.Parse(server.URL)
	client.Jar.SetCookies(serverURL, []*http.Cookie{{Name: "secret", Value: "credential"}})
	client.CheckRedirect = func(r *http.Request, via []*http.Request) error {
		r.Header.Set("Authorization", "secret")
		return nil
	}
	err := update.NewDownloader(client).Download(context.Background(), trustedManifest(t, server.URL+"/secret?token=hidden"), filepath.Join(t.TempDir(), "Ferry.zip"))
	if err != nil || requests.Load() != 2 {
		t.Fatalf("redirect result = %v, requests = %d", err, requests.Load())
	}
}

func TestDownloadRejectsUnsafeRedirects(t *testing.T) {
	var leaked atomic.Int32
	insecure := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { leaked.Add(1) }))
	defer insecure.Close()
	cases := []struct{ name, location string }{
		{"http", insecure.URL},
		{"userinfo", "https://user:secret@example.com/a"},
		{"fragment", "https://example.com/a#secret"},
		{"scheme", "file:///tmp/artifact"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := artifactServer(t, func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, tc.location, http.StatusFound) })
			directory := t.TempDir()
			err := update.NewDownloader(server.Client()).Download(context.Background(), trustedManifest(t, server.URL), filepath.Join(directory, "Ferry.zip"))
			if err == nil || strings.Contains(err.Error(), "secret") {
				t.Fatalf("unsafe redirect result = %v", err)
			}
			assertDirectoryEmpty(t, directory)
		})
	}
	if leaked.Load() != 0 {
		t.Fatal("followed HTTPS redirect to HTTP")
	}
}

func TestDownloadBoundsRedirectLoop(t *testing.T) {
	var count atomic.Int32
	server := artifactServer(t, func(w http.ResponseWriter, r *http.Request) {
		count.Add(1)
		http.Redirect(w, r, "/again", http.StatusFound)
	})
	directory := t.TempDir()
	err := update.NewDownloader(server.Client()).Download(context.Background(), trustedManifest(t, server.URL), filepath.Join(directory, "Ferry.zip"))
	if err == nil || count.Load() > 5 {
		t.Fatalf("redirect loop = %d requests: %v", count.Load(), err)
	}
	assertDirectoryEmpty(t, directory)
}
