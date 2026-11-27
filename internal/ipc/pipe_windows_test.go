//go:build windows

package ipc

import (
	"context"
	"testing"
	"time"
)

func TestNamedPipeRoundTripAndCredentialIsolation(t *testing.T) {
	credentials, err := NewCredentials()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	listener, err := Listen(credentials.Pipe)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- Serve(ctx, listener, NewHandler(credentials.Token, &testBackend{mode: "rule"})) }()
	client := NewClient(credentials)
	defer client.Close()
	state, err := client.Status(ctx)
	if err != nil || state.Mode != "rule" {
		t.Fatalf("round trip: %+v %v", state, err)
	}
	bad := credentials
	bad.Token = "wrong"
	rejected := NewClient(bad)
	defer rejected.Close()
	if _, err = rejected.Status(ctx); err == nil {
		t.Fatal("accepted wrong credential")
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("server did not shut down")
	}
}

func TestRejectArbitraryPipeNames(t *testing.T) {
	for _, name := range []string{`\\other\pipe\ferry-core-test`, `\\.\pipe\foreign`, `C:\\file`, ""} {
		if listener, err := Listen(name); err == nil {
			listener.Close()
			t.Fatalf("accepted pipe %q", name)
		}
	}
}
