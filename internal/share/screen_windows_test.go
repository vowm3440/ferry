//go:build windows

package share

import (
	"bytes"
	"image"
	"testing"
)

func TestDecodeQRImageRoundTripAndBounds(t *testing.T) {
	if _, err := DecodeQRImage(nil); err == nil {
		t.Fatal("nil image must fail")
	}
	pngData, err := QRPNG("socks5://127.0.0.1:1080#test")
	if err != nil {
		t.Fatal(err)
	}
	img, _, err := image.Decode(bytes.NewReader(pngData))
	if err != nil {
		t.Fatal(err)
	}
	text, err := DecodeQRImage(img)
	if err != nil || text != "socks5://127.0.0.1:1080#test" {
		t.Fatalf("DecodeQRImage mismatch: %q %v", text, err)
	}
	// Read-only GDI virtual screen capture should return a non-empty image or clean error.
	if shot, err := captureVirtualDesktop(); err == nil {
		if shot.Bounds().Dx() < 1 || shot.Bounds().Dy() < 1 {
			t.Fatal("empty desktop capture bounds")
		}
	}
}
