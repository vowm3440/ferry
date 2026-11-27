package update_test

import (
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"strings"
	"testing"

	"ferry/internal/update"
)

const fixtureHash = "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"

func manifestJSON(version, channel, url string, size int64) []byte {
	return []byte(fmt.Sprintf(`{"version":%q,"channel":%q,"url":%q,"sha256":%q,"size":%d}`,
		version, channel, url, fixtureHash, size))
}

func signingKey(t *testing.T) (ed25519.PublicKey, ed25519.PrivateKey) {
	t.Helper()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return public, private
}

func verifyFixture(t *testing.T, raw []byte, current, channel string) (update.Manifest, error) {
	t.Helper()
	public, private := signingKey(t)
	return update.VerifyManifest(raw, ed25519.Sign(private, raw), public, current, channel)
}

func TestVerifyManifestAcceptsSignedRawDocument(t *testing.T) {
	raw := append(manifestJSON("1.2.0", "stable", "https://example.com/ferry.zip", 5), '\n')
	manifest, err := verifyFixture(t, raw, "1.1.9", "stable")
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Version() != "1.2.0" || manifest.Channel() != "stable" || manifest.Size() != 5 {
		t.Fatalf("unexpected verified metadata: %v", manifest)
	}
	if manifest.URL() != "https://example.com/ferry.zip" || manifest.SHA256() != fixtureHash {
		t.Fatal("verified artifact metadata changed")
	}
}

func TestVerifyManifestRejectsInvalidSignatures(t *testing.T) {
	public, private := signingKey(t)
	otherPublic, _ := signingKey(t)
	raw := manifestJSON("1.2.0", "stable", "https://example.com/ferry.zip", 5)
	signature := ed25519.Sign(private, raw)
	cases := []struct {
		name           string
		raw, signature []byte
		public         ed25519.PublicKey
	}{
		{"tampered", append(append([]byte{}, raw...), ' '), signature, public},
		{"wrong-key", raw, signature, otherPublic},
		{"missing-key", raw, signature, nil},
		{"short-key", raw, signature, public[:1]},
		{"missing-signature", raw, nil, public},
		{"short-signature", raw, signature[:1], public},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := update.VerifyManifest(tc.raw, tc.signature, tc.public, "1.0.0", "stable"); err == nil {
				t.Fatal("invalid signature was accepted")
			}
		})
	}
}

func TestVerifyManifestRejectsAmbiguousJSON(t *testing.T) {
	base := string(manifestJSON("1.2.0", "stable", "https://example.com/ferry.zip", 5))
	cases := map[string]string{
		"unknown":           strings.TrimSuffix(base, "}") + `,"extra":true}`,
		"duplicate":         strings.TrimSuffix(base, "}") + `,"version":"1.2.0"}`,
		"escaped-duplicate": strings.TrimSuffix(base, "}") + `,"\u0076ersion":"1.2.0"}`,
		"case-alias":        strings.Replace(base, `"version"`, `"Version"`, 1),
		"trailing-object":   base + `{}`,
		"trailing-junk":     base + `x`,
		"array":             `[` + base + `]`,
		"null":              `null`,
		"null-value":        strings.Replace(base, `"1.2.0"`, `null`, 1),
		"fractional-size":   strings.Replace(base, `"size":5`, `"size":5.0`, 1),
		"string-size":       strings.Replace(base, `"size":5`, `"size":"5"`, 1),
		"missing":           strings.Replace(base, `"channel":"stable",`, "", 1),
		"oversized":         base + strings.Repeat(" ", 16*1024),
		"invalid-utf8":      strings.Replace(base, "stable", "sta\xffble", 1),
		"lone-surrogate":    strings.Replace(base, "ferry.zip", `\ud800.zip`, 1),
		"low-surrogate":     strings.Replace(base, "ferry.zip", `\udc00.zip`, 1),
		"reversed-pair":     strings.Replace(base, "ferry.zip", `\udc00\ud800.zip`, 1),
		"not-low-surrogate": strings.Replace(base, "ferry.zip", `\ud800\u0041.zip`, 1),
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := verifyFixture(t, []byte(raw), "1.0.0", "stable"); err == nil {
				t.Fatal("ambiguous JSON was accepted")
			}
		})
	}
}

func TestVerifyManifestPreservesValidEscapedUnicode(t *testing.T) {
	base := string(manifestJSON("1.2.0", "stable", "https://example.com/ferry.zip", 5))
	cases := []struct{ escaped, expected string }{
		{`\ud83d\udea2.zip`, "🚢.zip"},
		{`\ufffd.zip`, "�.zip"},
		{`\\ud800.zip`, `\ud800.zip`},
	}
	for _, tc := range cases {
		raw := strings.Replace(base, "ferry.zip", tc.escaped, 1)
		m, err := verifyFixture(t, []byte(raw), "1.0.0", "stable")
		if err != nil || m.URL() != "https://example.com/"+tc.expected {
			t.Fatalf("escaped URL %q = %q, %v", tc.escaped, m.URL(), err)
		}
	}
}

func TestVerifyManifestRejectsUnsafeMetadata(t *testing.T) {
	cases := []struct {
		name, channel, url, hash string
		size                     int64
	}{
		{"wrong-channel", "beta", "https://example.com/a", fixtureHash, 5},
		{"empty-channel", "", "https://example.com/a", fixtureHash, 5},
		{"http", "stable", "http://example.com/a", fixtureHash, 5},
		{"userinfo", "stable", "https://user:secret@example.com/a", fixtureHash, 5},
		{"fragment", "stable", "https://example.com/a#token", fixtureHash, 5},
		{"no-host", "stable", "https:///a", fixtureHash, 5},
		{"relative", "stable", "/a", fixtureHash, 5},
		{"empty-size", "stable", "https://example.com/a", fixtureHash, 0},
		{"negative-size", "stable", "https://example.com/a", fixtureHash, -1},
		{"oversized", "stable", "https://example.com/a", fixtureHash, 512*1024*1024 + 1},
		{"invalid-hash", "stable", "https://example.com/a", strings.Repeat("z", 64), 5},
		{"short-hash", "stable", "https://example.com/a", "1234", 5},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			raw := strings.Replace(string(manifestJSON("1.2.0", tc.channel, tc.url, tc.size)), fixtureHash, tc.hash, 1)
			if _, err := verifyFixture(t, []byte(raw), "1.0.0", "stable"); err == nil {
				t.Fatal("unsafe artifact was accepted")
			}
		})
	}
}
