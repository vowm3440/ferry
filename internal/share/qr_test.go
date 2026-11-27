package share

import (
	"strings"
	"testing"
)

func TestQRImageRoundTripAndSafeSVG(t *testing.T) {
	payload := "vless://11111111-2222-4333-8444-555555555555@edge.example.com:443?security=tls#test"
	image, err := QRPNG(payload)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeQR(image)
	if err != nil || decoded != payload {
		t.Fatalf("QR roundtrip failed: %v", err)
	}
	svg, err := QRSVG("<script>alert(1)</script>", 256)
	if err != nil || strings.Contains(svg, "<script>") {
		t.Fatal("payload injected into SVG")
	}
	if _, err := QRSVG(strings.Repeat("x", 5000), 256); err == nil {
		t.Fatal("oversized QR payload accepted")
	}
	if _, err := DecodeQR([]byte("not an image")); err == nil {
		t.Fatal("invalid image accepted")
	}
}
