//go:build windows

package instance

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"testing"
	"time"
)

func TestDeepLinkPipeForwarding(t *testing.T) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		t.Fatal(err)
	}
	_, sid, err := deepLinkPipeName()
	if err != nil {
		t.Fatal(err)
	}
	pipe := `\\.\pipe\ferry-deeplink-test-` + hex.EncodeToString(b[:])
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	got := make(chan string, 1)
	if err := listenDeepLinksPipe(ctx, pipe, sid, func(raw string) { got <- raw }); err != nil {
		t.Fatalf("listenDeepLinksPipe failed: %v", err)
	}
	want := "ferry://install-config?url=https%3A%2F%2Fexample.com%2Fsub"
	if err := forwardDeepLinkPipe(ctx, pipe, want); err != nil {
		t.Fatalf("forwardDeepLinkPipe failed: %v", err)
	}
	select {
	case actual := <-got:
		if actual != want {
			t.Fatalf("got %q, want %q", actual, want)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for forwarded deep link")
	}
	if err := forwardDeepLinkPipe(ctx, pipe, "bad\nline"); err == nil {
		t.Fatal("expected newline injection to be rejected")
	}
	if err := forwardDeepLinkPipe(ctx, pipe+"-missing", want); err == nil {
		t.Fatal("expected missing pipe to fail")
	}
	if lock, err := Acquire(); err == nil {
		_ = lock.Close()
	}
}
