package share

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestLANRateLimitsWrongTokensAndUnsupportedMethods(t *testing.T) {
	for _, method := range []string{http.MethodGet, http.MethodHead} {
		t.Run(method, func(t *testing.T) {
			server := newLoopbackLAN(t, 0)
			info := startLAN(t, server)
			client := lanClient(t)
			limited := false
			for range 32 {
				status, body, _ := readLAN(t, client, info.URL+"-wrong", method)
				if strings.Contains(body, "private") {
					t.Fatal("rejected request exposed sharing content")
				}
				limited = limited || status == http.StatusTooManyRequests
			}
			if !limited || !server.Status().Active {
				t.Fatal("invalid requests bypassed rate limit or consumed the token")
			}
		})
	}
}

func TestLANActiveStartCannotReplaceSnapshot(t *testing.T) {
	server := newLoopbackLAN(t, 0)
	info := startLAN(t, server)
	if _, err := server.Start(context.Background(), []byte("replacement"), "text/plain"); err == nil {
		t.Fatal("start silently replaced existing sharing session")
	}
	status, body, _ := readLAN(t, lanClient(t), info.URL, http.MethodGet)
	if status != http.StatusOK || body != "private configuration" {
		t.Fatal("failed replacement changed original snapshot")
	}
}

func TestLANRejectsUnsafeBindingsAndMIMETypes(t *testing.T) {
	for _, address := range []string{"0.0.0.0", "::", "localhost", "example.com", "239.0.0.1", "127.0.0.1:80"} {
		if server, err := NewLAN(LANOptions{BindIP: address}); err == nil {
			server.Close()
			t.Fatal("non-specific IP binding accepted")
		}
	}
	for _, ttl := range []time.Duration{-time.Second, 11 * time.Minute} {
		if server, err := NewLAN(LANOptions{TTL: ttl}); err == nil {
			server.Close()
			t.Fatal("unsafe sharing lifetime accepted")
		}
	}
	server := newLoopbackLAN(t, 0)
	for _, contentType := range []string{"text/html", "application/javascript", "text/plain\r\nX: secret", "text/plain; filename=secret", "text/plain; charset=utf-16"} {
		if _, err := server.Start(context.Background(), []byte("secret"), contentType); err == nil {
			t.Fatal("unsafe MIME type accepted")
		}
	}
	remote, err := NewLAN(LANOptions{BindIP: "192.0.2.123"})
	if err != nil {
		t.Fatal(err)
	}
	defer remote.Close()
	if _, err := remote.Start(context.Background(), []byte("secret"), "text/plain"); err == nil {
		t.Fatal("remote IP accepted as a sharing listener")
	}
}

func TestLANRequestBodyAndCanceledRequestDoNotConsumeToken(t *testing.T) {
	server := newLoopbackLAN(t, 0)
	info := startLAN(t, server)
	request, _ := http.NewRequest(http.MethodGet, info.URL, strings.NewReader("unexpected input"))
	response, err := lanClient(t).Do(request)
	if err != nil {
		t.Fatal("body rejection request failed")
	}
	response.Body.Close()
	if response.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatal("request body accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	request, _ = http.NewRequestWithContext(ctx, http.MethodGet, info.URL, nil)
	recorder := httptest.NewRecorder()
	server.current.ServeHTTP(recorder, request)
	if recorder.Body.Len() != 0 {
		t.Fatal("canceled request received sharing content")
	}
	status, body, _ := readLAN(t, lanClient(t), info.URL, http.MethodGet)
	if status != http.StatusOK || body != "private configuration" {
		t.Fatal("rejected request consumed sharing token")
	}
}

func TestLANTransfersMaximumSizeAndIgnoresUntrustedHost(t *testing.T) {
	server := newLoopbackLAN(t, 0)
	payload := strings.Repeat("x", 1<<20)
	info, err := server.Start(context.Background(), []byte(payload), "")
	if err != nil {
		t.Fatal(err)
	}
	request, _ := http.NewRequest(http.MethodGet, info.URL, nil)
	request.Host = "attacker.example"
	response, err := lanClient(t).Do(request)
	if err != nil {
		t.Fatal("maximum payload request failed")
	}
	defer response.Body.Close()
	data, err := io.ReadAll(response.Body)
	if err != nil || string(data) != payload || response.StatusCode != http.StatusOK {
		t.Fatal("maximum payload transfer was interrupted")
	}
	if response.Header.Get("Content-Type") != "application/octet-stream" {
		t.Fatal("missing safe default MIME type")
	}
	waitLANClosed(t, info)
}

func TestLANRejectsExcessIncompleteConnections(t *testing.T) {
	server := newLoopbackLAN(t, 0)
	info := startLAN(t, server)
	parsed, _ := url.Parse(info.URL)
	for range 8 {
		connection, err := net.DialTimeout("tcp", parsed.Host, time.Second)
		if err != nil {
			t.Fatal("bounded connection setup failed")
		}
		defer connection.Close()
		if _, err := io.WriteString(connection, "GET / HTTP/1.1\r\n"); err != nil {
			t.Fatal(err)
		}
	}
	response, err := lanClient(t).Get(info.URL)
	if err == nil {
		response.Body.Close()
		t.Fatal("connection limit did not reject excess connection")
	}
	if !server.Status().Active {
		t.Fatal("over-limit connection consumed sharing token")
	}
}

func TestLANOversizedHeadersDoNotExposeContent(t *testing.T) {
	server := newLoopbackLAN(t, 0)
	info := startLAN(t, server)
	request, _ := http.NewRequest(http.MethodGet, info.URL, nil)
	request.Header.Set("X-Padding", strings.Repeat("x", 16<<10))
	response, err := lanClient(t).Do(request)
	if err != nil {
		t.Fatal("header rejection request failed")
	}
	defer response.Body.Close()
	// Windows may reset a rejected connection that still has unread input.
	data, _ := io.ReadAll(response.Body)
	if response.StatusCode != http.StatusRequestHeaderFieldsTooLarge || strings.Contains(string(data), "private") {
		t.Fatal("oversized headers were not rejected safely")
	}
	if !server.Status().Active {
		t.Fatal("oversized headers consumed sharing token")
	}
}
