//go:build ignore

// Command sign-release generates a signed update manifest and .sig file for GitHub Releases.
// Usage:
//   go run ./scripts/sign-release.go -key .keys/release-ed25519.key -version 0.2.0 -channel stable -file dist/Ferry-0.2.0-windows-amd64-setup.exe -url https://github.com/vowm3440/ferry/releases/download/v0.2.0/Ferry-0.2.0-windows-amd64-setup.exe
package main

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type manifestDoc struct {
	Version string `json:"version"`
	Channel string `json:"channel"`
	URL     string `json:"url"`
	SHA256  string `json:"sha256"`
	Size    int64  `json:"size"`
}

func main() {
	keyPath := flag.String("key", ".keys/release-ed25519.key", "path to base64-encoded ed25519 private key")
	version := flag.String("version", "", "SemVer version without 'v' prefix (e.g. 0.2.0)")
	channel := flag.String("channel", "stable", "update channel (stable or beta)")
	artifact := flag.String("file", "", "path to release installer or zip file")
	downloadURL := flag.String("url", "", "HTTPS download URL of the artifact")
	outDir := flag.String("out", "dist", "directory to write manifest-<channel>.json and .sig")
	flag.Parse()

	if *version == "" || *artifact == "" || *downloadURL == "" {
		fmt.Fprintln(os.Stderr, "missing required flags: -version, -file, -url")
		os.Exit(2)
	}
	priv, err := loadPrivateKey(*keyPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	digest, size, err := hashArtifact(*artifact)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := writeSignedManifest(*outDir, *channel, manifestDoc{
		Version: *version, Channel: *channel, URL: *downloadURL, SHA256: digest, Size: size,
	}, priv); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func loadPrivateKey(path string) (ed25519.PrivateKey, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read private key: %w", err)
	}
	key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(raw)))
	if err != nil || len(key) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("invalid ed25519 private key length in %s", path)
	}
	return ed25519.PrivateKey(key), nil
}

func hashArtifact(path string) (string, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()
	h := sha256.New()
	size, err := io.Copy(h, f)
	if err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(h.Sum(nil)), size, nil
}

func writeSignedManifest(outDir, channel string, doc manifestDoc, priv ed25519.PrivateKey) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	sig := base64.StdEncoding.EncodeToString(ed25519.Sign(priv, raw)) + "\n"
	manifestPath := filepath.Join(outDir, "manifest-"+channel+".json")
	sigPath := manifestPath + ".sig"
	if err := os.WriteFile(manifestPath, raw, 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(sigPath, []byte(sig), 0o644); err != nil {
		return err
	}
	fmt.Printf("Signed %s and %s\n", manifestPath, sigPath)
	return nil
}
