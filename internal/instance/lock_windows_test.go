package instance

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"testing"
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
