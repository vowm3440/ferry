package instance

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"testing"
	"time"
)

func TestOnlyOneOwnerUntilHandleCloses(t *testing.T) {
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		t.Fatal(err)
	}
	name := `Local\Ferry.Test.` + hex.EncodeToString(id[:])
	first, err := acquire(name)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { first.Close() })
	if second, err := acquire(name); !errors.Is(err, ErrRunning) {
		if second != nil {
			second.Close()
		}
		t.Fatalf("second instance not excluded: %v", err)
	}
	if err = first.Close(); err != nil {
		t.Fatal(err)
	}
	next, err := acquire(name)
	if err != nil {
		t.Fatal(err)
	}
	if err := next.Close(); err != nil {
		t.Fatal(err)
	}
	if err := next.Close(); err != nil {
		t.Fatalf("second Close must be idempotent: %v", err)
	}
	if _, err := acquire("bad\x00name"); err == nil {
		t.Fatal("null-byte mutex name must fail")
	}
}

func TestAcquireWithTimeoutWaitsForPreviousInstanceRelease(t *testing.T) {
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		t.Fatal(err)
	}
	name := `Local\Ferry.TimeoutTest.` + hex.EncodeToString(id[:])
	first, err := acquire(name)
	if err != nil {
		t.Fatal(err)
	}
	// Timeout while still held returns ErrRunning.
	if second, err := acquireWithTimeout(name, 150*time.Millisecond); !errors.Is(err, ErrRunning) {
		if second != nil {
			_ = second.Close()
		}
		_ = first.Close()
		t.Fatalf("expected ErrRunning on timeout, got %v", err)
	}
	go func() {
		time.Sleep(150 * time.Millisecond)
		_ = first.Close()
	}()
	next, err := acquireWithTimeout(name, 2*time.Second)
	if err != nil {
		_ = first.Close()
		t.Fatalf("acquireWithTimeout should succeed after lock release: %v", err)
	}
	_ = next.Close()
}
