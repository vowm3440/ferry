// Package update verifies signed update metadata and stages verified downloads.
// It does not configure an update service, install, or execute downloaded files.
package update

import (
	"crypto/ed25519"
	"encoding/hex"
	"errors"
	"net/url"
	"regexp"
	"strings"
	"unicode/utf8"
)

// MaxManifestBytes bounds the complete, signed JSON document before parsing.
const MaxManifestBytes = 16 * 1024

// MaxDownloadBytes bounds the declared and received artifact size.
const MaxDownloadBytes int64 = 512 * 1024 * 1024

var channelPattern = regexp.MustCompile(`^[a-z][a-z0-9-]{0,31}$`)

// Manifest is immutable update metadata returned only by VerifyManifest.
// Its zero value is invalid and cannot authorize a download.
type Manifest struct {
	version, channel, url, sha256 string
	size                          int64
	verified                      bool
}

func (m Manifest) Version() string { return m.version }
func (m Manifest) Channel() string { return m.channel }
func (m Manifest) URL() string     { return m.url }
func (m Manifest) SHA256() string  { return m.sha256 }
func (m Manifest) Size() int64     { return m.size }

// VerifyManifest authenticates the exact raw bytes with a caller-pinned key.
// JSON must contain exactly version, channel, url, sha256, and size. Versions
// use SemVer 2.0 without a "v" prefix; equal versions and downgrades are rejected.
func VerifyManifest(raw, signature []byte, publicKey ed25519.PublicKey, currentVersion, channel string) (Manifest, error) {
	if len(raw) == 0 || len(raw) > MaxManifestBytes || !utf8.Valid(raw) {
		return Manifest{}, errors.New("invalid update manifest size or encoding")
	}
	if len(publicKey) != ed25519.PublicKeySize || len(signature) != ed25519.SignatureSize || !ed25519.Verify(publicKey, raw, signature) {
		return Manifest{}, errors.New("update manifest signature verification failed")
	}
	m, err := decodeManifest(raw)
	if err != nil {
		return Manifest{}, err
	}
	if err = validateManifest(m, currentVersion, channel); err != nil {
		return Manifest{}, err
	}
	m.sha256 = strings.ToLower(m.sha256)
	m.verified = true
	return m, nil
}

func validateManifest(m Manifest, currentVersion, channel string) error {
	if !channelPattern.MatchString(channel) || m.channel != channel {
		return errors.New("update manifest channel mismatch")
	}
	if !newerVersion(m.version, currentVersion) {
		return errors.New("update version must be valid and newer than the current version")
	}
	if _, err := artifactURL(m.url); err != nil {
		return err
	}
	digest, err := hex.DecodeString(m.sha256)
	if err != nil || len(digest) != 32 {
		return errors.New("invalid update SHA-256 digest")
	}
	if m.size <= 0 || m.size > MaxDownloadBytes {
		return errors.New("invalid update download size")
	}
	return nil
}

func artifactURL(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil || len(raw) > 4096 || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.Opaque != "" || strings.Contains(raw, "#") {
		return nil, errors.New("update download requires an HTTPS URL without credentials or fragments")
	}
	return u, nil
}
