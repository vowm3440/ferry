package update_test

import (
	"crypto/ed25519"
	"testing"

	"ferry/internal/update"
)

func FuzzVerifyManifestRejectsMalformedSignedInput(f *testing.F) {
	public, private, err := ed25519.GenerateKey(nil)
	if err != nil {
		f.Fatal(err)
	}
	f.Add(manifestJSON("1.2.0", "stable", "https://example.com/a", 5))
	f.Add([]byte(`{"version":"1.2.0","version":"2.0.0"}`))
	f.Add([]byte(`{"url":"https://example.com/\ud800"}`))
	f.Fuzz(func(t *testing.T, raw []byte) {
		if len(raw) > update.MaxManifestBytes+1 {
			return
		}
		m, err := update.VerifyManifest(raw, ed25519.Sign(private, raw), public, "1.0.0", "stable")
		if err == nil && (m.Channel() != "stable" || m.Size() <= 0 || m.Size() > update.MaxDownloadBytes) {
			t.Fatal("accepted invalid artifact metadata")
		}
	})
}
