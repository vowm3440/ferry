//go:build windows

package wintrust

import (
	"bytes"
	"errors"
	"runtime"
	"testing"
	"unsafe"
)

func TestSignerMemoryRejectsMissingAndUnreadableAddresses(t *testing.T) {
	if _, err := signerDERFromState(0); !errors.Is(err, ErrSignerCertificate) {
		t.Fatalf("empty trust state error = %v", err)
	}
	for _, address := range []uintptr{0, 1} {
		if _, err := signerDERFromContext(address); !errors.Is(err, ErrSignerCertificate) {
			t.Fatalf("unreadable certificate context error = %v", err)
		}
	}
}

func TestSignerMemoryRejectsInvalidCertificateLength(t *testing.T) {
	cases := []certificateContext{
		{}, {encoded: 1}, {encoded: 1, length: 1 << 21}, {encoded: 1, length: 1},
	}
	for _, context := range cases {
		if _, err := signerDERFromContext(uintptr(unsafe.Pointer(&context))); !errors.Is(err, ErrSignerCertificate) {
			t.Fatalf("invalid native certificate error = %v", err)
		}
		runtime.KeepAlive(context)
	}
}

func TestSignerMemoryCopiesBeforeNativeOwnerIsReleased(t *testing.T) {
	source := []byte{1, 2, 3, 4}
	context := certificateContext{encoded: uintptr(unsafe.Pointer(&source[0])), length: uint32(len(source))}
	der, err := signerDERFromContext(uintptr(unsafe.Pointer(&context)))
	runtime.KeepAlive(context)
	if err != nil {
		t.Fatal(err)
	}
	source[0] = 99
	if !bytes.Equal(der, []byte{1, 2, 3, 4}) {
		t.Fatal("signer DER aliases native-owned memory")
	}
	runtime.KeepAlive(source)
}
