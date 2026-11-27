//go:build windows

package wintrust

import (
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"golang.org/x/sys/windows"
)

func TestNativeSignedFixtureRequiresPinnedPublisher(t *testing.T) {
	path := signedFixturePath(t)
	locked, err := openLockedFile(path)
	if err != nil {
		t.Fatal(err)
	}
	defer locked.close()
	der, err := verifiedSignerCertificate(locked.finalPath, locked.handle)
	if err != nil {
		if unavailableNativeTrust(err) {
			t.Skipf("系统信任或离线吊销缓存不可用，未验收真实 signer 提取：%v", err)
		}
		t.Fatalf("native signer extraction: %v", err)
	}
	certificate, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(certificate.RawSubjectPublicKeyInfo)
	pin := hex.EncodeToString(hash[:])
	file, err := Verify(path, []string{pin})
	if err != nil {
		t.Fatalf("native pinned signature: %v", err)
	}
	defer file.Close()
	if file.SignerSPKI() != pin || file.FileID() != locked.identity || file.FinalPath() != locked.finalPath {
		t.Fatal("verified identity differs from the locked native signer")
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	wrong, err := Verify(path, []string{strings.Repeat("0", 64)})
	if wrong != nil || !errors.Is(err, ErrPublisherMismatch) {
		t.Fatalf("trusted wrong publisher result = %v, %v", wrong, err)
	}
}

func signedFixturePath(t *testing.T) string {
	t.Helper()
	// The stack frame is an x/sys/windows function, locating its module without
	// invoking the fixture or relying on a caller-provided signature bypass.
	pc := runtime.FuncForPC(reflect.ValueOf(windows.GetFileType).Pointer())
	source, _ := pc.FileLine(pc.Entry())
	path := filepath.Join(filepath.Dir(source), "testdata", "ev-signed-file.exe")
	if _, err := os.Stat(path); err != nil {
		t.Skip("只读 x/sys EV 签名样例在当前构建中不可定位")
	}
	return path
}

func unavailableNativeTrust(err error) bool {
	var status *nativeTrustError
	if !errors.As(err, &status) {
		return false
	}
	switch windows.Handle(status.code) {
	case windows.CERT_E_UNTRUSTEDROOT, windows.CERT_E_CHAINING,
		windows.CRYPT_E_REVOCATION_OFFLINE, windows.CERT_E_REVOCATION_FAILURE:
		return true
	}
	return false
}

func TestVerifyRejectsUnsignedFileAndReleasesHandle(t *testing.T) {
	path := temporaryExecutable(t)
	file, err := Verify(path, []string{strings.Repeat("0", 64)})
	if file != nil || !errors.Is(err, ErrTrust) {
		t.Fatalf("unsigned file result = %v, %v", file, err)
	}
	if strings.Contains(err.Error(), path) {
		t.Fatal("file path leaked into error")
	}
	if err := os.Remove(path); err != nil {
		t.Fatalf("unsigned file handle was not released: %v", err)
	}
}

func TestLockedFileDeniesWriteAndDeleteSharing(t *testing.T) {
	path := temporaryExecutable(t)
	file, err := openLockedFile(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.close()
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, access := range []uint32{windows.GENERIC_WRITE, windows.DELETE} {
		handle, err := windows.CreateFile(name, access, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING, 0, 0)
		if err == nil {
			windows.CloseHandle(handle)
			t.Fatalf("locked file permitted access %#x", access)
		}
		if !errors.Is(err, windows.ERROR_SHARING_VIOLATION) {
			t.Fatalf("unexpected sharing error: %v", err)
		}
	}
}

func TestLockedFileRecordsFinalPathAndSameFileIdentity(t *testing.T) {
	path := temporaryExecutable(t)
	first, err := openLockedFile(path)
	if err != nil {
		t.Fatal(err)
	}
	defer first.close()
	second, err := openLockedFile(filepath.Join(filepath.Dir(path), ".", filepath.Base(path)))
	if err != nil {
		t.Fatal(err)
	}
	defer second.close()
	if !strings.EqualFold(first.finalPath, path) || first.identity != second.identity {
		t.Fatal("locked handles did not retain final path and file identity")
	}
	if first.identity.Index == 0 {
		t.Fatal("missing file index")
	}
}

func TestVerifiedFileCloseReleasesRealLockOnceAcrossCopies(t *testing.T) {
	path := temporaryExecutable(t)
	locked, err := openLockedFile(path)
	if err != nil {
		t.Fatal(err)
	}
	verified := &VerifiedFile{release: &fileRelease{close: locked.close}}
	defer verified.Close()
	copyOfVerified := *verified
	if err := verified.Close(); err != nil {
		t.Fatal(err)
	}
	if err := copyOfVerified.Close(); err != nil {
		t.Fatalf("copied verification closed the same native handle twice: %v", err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatalf("Close did not release the real file lock: %v", err)
	}
}

func TestLockedFileRejectsFinalReparsePoint(t *testing.T) {
	path := temporaryExecutable(t)
	link := filepath.Join(filepath.Dir(path), "link.exe")
	if err := os.Symlink(path, link); err != nil {
		if errors.Is(err, windows.ERROR_PRIVILEGE_NOT_HELD) {
			t.Skip("当前用户没有创建测试符号链接的权限，末级 reparse 拒绝未获原生验收")
		}
		t.Fatalf("create test symlink: %v", err)
	}
	file, err := openLockedFile(link)
	if file != nil {
		file.close()
	}
	if !errors.Is(err, ErrUnsafeFile) {
		t.Fatalf("final reparse point error = %v", err)
	}
}

func temporaryExecutable(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "unsigned-private-name.exe")
	if err := os.WriteFile(path, []byte("not an Authenticode executable"), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}
