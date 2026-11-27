package wintrust

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"errors"
	"math/big"
	"strings"
	"testing"
	"time"
)

func TestPinsRejectMissingAndMalformedIdentity(t *testing.T) {
	cases := []struct {
		name string
		pins []string
		want error
	}{
		{"nil", nil, ErrMissingPins},
		{"empty", []string{}, ErrMissingPins},
		{"empty entry", []string{""}, ErrInvalidPin},
		{"short", []string{strings.Repeat("a", 63)}, ErrInvalidPin},
		{"long", []string{strings.Repeat("a", 65)}, ErrInvalidPin},
		{"non hex", []string{strings.Repeat("g", 64)}, ErrInvalidPin},
		{"space", []string{" " + strings.Repeat("a", 64)}, ErrInvalidPin},
		{"prefix", []string{"sha256:" + strings.Repeat("a", 64)}, ErrInvalidPin},
		{"mixed valid invalid", []string{strings.Repeat("a", 64), "bad"}, ErrInvalidPin},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := parsePins(tc.pins); !errors.Is(err, tc.want) {
				t.Fatalf("parsePins error = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestSignerDERRequiresPinnedCodeSigningKey(t *testing.T) {
	der, pin := testCertificate(t, []x509.ExtKeyUsage{x509.ExtKeyUsageCodeSigning}, x509.KeyUsageDigitalSignature)
	pins, err := parsePins([]string{strings.Repeat("0", 64), strings.ToUpper(pin)})
	if err != nil {
		t.Fatal(err)
	}
	got, err := validateSignerDER(der, pins)
	if err != nil || got != pin {
		t.Fatalf("signer pin = %q, error = %v", got, err)
	}
	wrongPins, err := parsePins([]string{strings.Repeat("0", 64)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := validateSignerDER(der, wrongPins); !errors.Is(err, ErrPublisherMismatch) {
		t.Fatalf("wrong publisher error = %v", err)
	}
	if _, err := validateSignerDER(der, nil); !errors.Is(err, ErrMissingPins) {
		t.Fatalf("missing publisher error = %v", err)
	}
}

func TestSignerDERRejectsWrongCertificatePurposes(t *testing.T) {
	cases := []struct {
		name string
		eku  []x509.ExtKeyUsage
		ku   x509.KeyUsage
	}{
		{"server auth", []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, x509.KeyUsageDigitalSignature},
		{"missing EKU", nil, x509.KeyUsageDigitalSignature},
		{"any EKU", []x509.ExtKeyUsage{x509.ExtKeyUsageAny}, x509.KeyUsageDigitalSignature},
		{"no digital signature", []x509.ExtKeyUsage{x509.ExtKeyUsageCodeSigning}, x509.KeyUsageKeyEncipherment},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			der, pin := testCertificate(t, tc.eku, tc.ku)
			pins, err := parsePins([]string{pin})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := validateSignerDER(der, pins); !errors.Is(err, ErrSignerCertificate) {
				t.Fatalf("invalid certificate purpose error = %v", err)
			}
		})
	}
}

func TestSignerDERRejectsMalformedData(t *testing.T) {
	pins, err := parsePins([]string{strings.Repeat("0", 64)})
	if err != nil {
		t.Fatal(err)
	}
	for _, der := range [][]byte{nil, {}, []byte("private certificate material"), {0x30, 0x02, 0x01}} {
		if _, err := validateSignerDER(der, pins); !errors.Is(err, ErrSignerCertificate) {
			t.Fatalf("malformed certificate error = %v", err)
		} else if strings.Contains(err.Error(), "private certificate material") {
			t.Fatal("certificate bytes leaked into error")
		}
	}
}

// These self-signed, already expired certificates test only DER and pin logic.
// They are never installed or passed off as a successful Windows trust result.
func testCertificate(t *testing.T, eku []x509.ExtKeyUsage, ku x509.KeyUsage) ([]byte, string) {
	t.Helper()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "Ferry test only"},
		NotBefore: time.Date(2001, 1, 1, 0, 0, 0, 0, time.UTC),
		NotAfter:  time.Date(2002, 1, 1, 0, 0, 0, 0, time.UTC),
		KeyUsage:  ku, ExtKeyUsage: eku,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, public, private)
	if err != nil {
		t.Fatal(err)
	}
	spki, err := x509.MarshalPKIXPublicKey(public)
	if err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(spki)
	return der, hex.EncodeToString(hash[:])
}
