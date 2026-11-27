package release

import (
	"crypto/ed25519"
	"errors"
	"strings"
	"testing"
)

func TestReleaseUpdateFeedAndDefaults(t *testing.T) {
	if len(SignerPins()) != 0 {
		t.Fatal("unexpected publisher identity")
	}
	feed, err := RequireUpdateFeed()
	if err != nil {
		t.Fatalf("expected configured update feed, got %v", err)
	}
	if !strings.HasPrefix(feed.ManifestURL, "https://github.com/vowm3440/ferry/") || len(feed.PublicKey) != ed25519.PublicKeySize {
		t.Fatalf("unexpected feed configuration: %+v", feed)
	}

	origURL, origKey := updateManifestURL, updatePublicKey
	t.Cleanup(func() {
		updateManifestURL, updatePublicKey = origURL, origKey
	})
	updateManifestURL = ""
	if _, err := RequireUpdateFeed(); !errors.Is(err, ErrUpdateUnconfigured) {
		t.Fatalf("expected ErrUpdateUnconfigured when URL is empty, got %v", err)
	}
}
