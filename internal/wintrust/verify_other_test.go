//go:build !windows

package wintrust

import (
	"errors"
	"strings"
	"testing"
)

func TestVerifyRejectsUnsupportedPlatform(t *testing.T) {
	file, err := Verify("unused.exe", []string{strings.Repeat("0", 64)})
	if file != nil || !errors.Is(err, ErrUnsupported) {
		t.Fatalf("unsupported platform result = %v, %v", file, err)
	}
}
