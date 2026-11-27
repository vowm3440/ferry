// Package wintrust verifies Windows Authenticode signatures on locked files.
// A trusted signature proves publisher identity, not a protected installation
// directory. Callers must separately authorize final paths, directory reparse
// points, ownership and DACLs, and keep VerifiedFile open while using identity.
package wintrust

import (
	"errors"
	"sync"
)

var (
	ErrTrust       = errors.New("发行签名系统信任验证失败")
	ErrUnsafeFile  = errors.New("发行文件类型不安全")
	ErrFileAccess  = errors.New("无法锁定或读取发行文件身份")
	ErrCleanup     = errors.New("发行签名验证资源释放失败")
	ErrUnsupported = errors.New("发行签名验证仅支持 Windows")
)

// FileIdentity is a volume serial number and file index captured from the
// locked handle. It is an identity snapshot, not an authorization decision.
type FileIdentity struct {
	VolumeSerialNumber uint32
	Index              uint64
}

// VerifiedFile retains the file lock until Close. Its identity getters return
// values, and never expose the handle or Windows-owned certificate memory.
type VerifiedFile struct {
	finalPath  string
	identity   FileIdentity
	signerSPKI string
	release    *fileRelease
}

type fileRelease struct {
	once  sync.Once
	close func() error
	err   error
}

// Verify requires trusted, compile-time publisher pins from its caller. Pins
// are exactly 64 hexadecimal characters; no prefix or whitespace is accepted.
// Missing or malformed pins are rejected before any OS or filesystem access.
// Application callers must never source pins from environment, IPC or config.
func Verify(path string, pins []string) (*VerifiedFile, error) {
	allowed, err := parsePins(pins)
	if err != nil {
		return nil, err
	}
	return verifyNative(path, allowed)
}

// FinalPath is the normalized final DOS path of the locked file handle.
func (file *VerifiedFile) FinalPath() string { return file.finalPath }

// FileID returns the locked file's identity by value.
func (file *VerifiedFile) FileID() FileIdentity { return file.identity }

// SignerSPKI is the signer's SPKI SHA-256 digest in lowercase hexadecimal.
func (file *VerifiedFile) SignerSPKI() string { return file.signerSPKI }

// Close releases the file lock. It is safe to call more than once. The identity
// snapshots remain readable, but must no longer authorize use of the path.
func (file *VerifiedFile) Close() error {
	if file == nil || file.release == nil {
		return nil
	}
	file.release.once.Do(func() { file.release.err = file.release.close() })
	return file.release.err
}
