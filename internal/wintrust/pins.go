package wintrust

import (
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"errors"
)

var (
	ErrMissingPins       = errors.New("未配置发行签名身份")
	ErrInvalidPin        = errors.New("发行签名公钥摘要格式无效")
	ErrSignerCertificate = errors.New("发行签名证书无效")
	ErrPublisherMismatch = errors.New("发行签名发布者不匹配")
)

type spkiPin [32]byte

func parsePins(pins []string) ([]spkiPin, error) {
	if len(pins) == 0 {
		return nil, ErrMissingPins
	}
	parsed := make([]spkiPin, len(pins))
	for i, pin := range pins {
		if len(pin) != sha256.Size*2 {
			return nil, ErrInvalidPin
		}
		if _, err := hex.Decode(parsed[i][:], []byte(pin)); err != nil {
			return nil, ErrInvalidPin
		}
	}
	return parsed, nil
}

// validateSignerDER adds publisher and purpose constraints to a successful
// WinVerifyTrust result; it is not an independent certificate trust verifier.
// Windows owns certificate lifetime and trusted timestamp policy.
func validateSignerDER(der []byte, pins []spkiPin) (string, error) {
	if len(pins) == 0 {
		return "", ErrMissingPins
	}
	certificate, err := x509.ParseCertificate(der)
	if err != nil || !permitsCodeSigning(certificate) {
		return "", ErrSignerCertificate
	}
	hash := spkiPin(sha256.Sum256(certificate.RawSubjectPublicKeyInfo))
	for _, pin := range pins {
		if hash == pin {
			return hex.EncodeToString(hash[:]), nil
		}
	}
	return "", ErrPublisherMismatch
}

func permitsCodeSigning(certificate *x509.Certificate) bool {
	if certificate.KeyUsage&x509.KeyUsageDigitalSignature == 0 {
		return false
	}
	for _, usage := range certificate.ExtKeyUsage {
		if usage == x509.ExtKeyUsageCodeSigning {
			return true
		}
	}
	return false
}
