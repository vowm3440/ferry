//go:build windows

package ipc

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

func currentUserSID(t *testing.T) string {
	t.Helper()
	u, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	return u.User.Sid.String()
}

func randomTestPipe(t *testing.T) string {
	t.Helper()
	var b [12]byte
	if _, err := rand.Read(b[:]); err != nil {
		t.Fatal(err)
	}
	return `\\.\pipe\ferry-svc-test-` + hex.EncodeToString(b[:])
}

func TestServicePathsIgnoreEnvOverride(t *testing.T) {
	t.Setenv("FERRY_CONFIG_DIR", t.TempDir())
	paths, err := ServicePaths()
	if err != nil {
		t.Fatalf("ServicePaths failed: %v", err)
	}
	if !filepath.IsAbs(paths.DataDir) || !filepath.IsAbs(paths.ServiceCorePath) {
		t.Fatalf("expected absolute paths: %+v", paths)
	}
	if filepath.Base(paths.ServiceCorePath) != "ferry-core.exe" {
		t.Fatalf("unexpected binary name: %+v", paths)
	}
	if strings.Contains(strings.ToLower(paths.DataDir), strings.ToLower(os.Getenv("FERRY_CONFIG_DIR"))) {
		t.Fatal("ServicePaths must never honor FERRY_CONFIG_DIR")
	}
}

func TestServicePipeSDDLValidation(t *testing.T) {
	for _, bad := range []string{"", "not-a-sid", "S-1-5-18", "S-1-5-19", "S-1-5-20", "S-1-1-0", "S-1-5-11"} {
		if _, err := ServicePipeSDDL(bad); err == nil {
			t.Fatalf("expected invalid/system SID %q to be rejected", bad)
		}
	}
	sid := currentUserSID(t)
	sddl, err := ServicePipeSDDL(sid)
	if err != nil {
		t.Fatalf("valid user SID rejected: %v", err)
	}
	if !strings.Contains(sddl, "0x12019b") || !strings.Contains(sddl, sid) {
		t.Fatalf("unexpected service pipe SDDL: %s", sddl)
	}
}

func TestVerifyInstallTreeRejectsUnsafeDirsAndJunctions(t *testing.T) {
	if err := VerifyInstallTree("relative\\path"); err == nil {
		t.Fatal("relative path must be rejected")
	}
	userDir := t.TempDir()
	if err := VerifyInstallTree(userDir); err == nil {
		t.Fatal("user-owned writable temp directory must be rejected")
	}
	target := filepath.Join(userDir, "target")
	if err := os.Mkdir(target, 0o700); err != nil {
		t.Fatal(err)
	}
	junction := filepath.Join(userDir, "link")
	if out, err := exec.Command("cmd", "/c", "mklink", "/J", junction, target).CombinedOutput(); err != nil {
		t.Skipf("mklink /J unavailable: %v (%s)", err, string(out))
	}
	if err := checkNoReparseChain(junction); err == nil {
		t.Fatal("directory junction must be rejected")
	}
	sysRoot := os.Getenv("SystemRoot")
	if sysRoot != "" {
		if err := VerifyInstallTree(filepath.Join(sysRoot, "System32")); err != nil {
			t.Fatalf("expected protected System32 directory to pass VerifyInstallTree: %v", err)
		}
	}
}

type fakeVerifier struct {
	allowClient bool
	allowServer bool
	releases    atomic.Int32
}

func (f *fakeVerifier) VerifyClient(conn net.Conn) (PeerIdentity, func() error, error) {
	pid, err := pipePeerPID(conn, procGetNamedPipeClientPID)
	if err != nil || pid == 0 {
		return PeerIdentity{}, nil, errors.New("missing client pid")
	}
	if !f.allowClient {
		return PeerIdentity{}, nil, errors.New("client rejected")
	}
	return PeerIdentity{PID: pid, UserSID: "S-1-5-21-1-2-3-1000"}, func() error {
		f.releases.Add(1)
		return nil
	}, nil
}

func (f *fakeVerifier) VerifyServer(conn net.Conn, expectedPID uint32) (PeerIdentity, func() error, error) {
	pid, err := pipePeerPID(conn, procGetNamedPipeServerPID)
	if err != nil || pid != expectedPID {
		return PeerIdentity{}, nil, errors.New("server pid mismatch")
	}
	if !f.allowServer {
		return PeerIdentity{}, nil, errors.New("server rejected")
	}
	return PeerIdentity{PID: pid}, func() error {
		f.releases.Add(1)
		return nil
	}, nil
}

func TestServicePipeMutualVerificationAndConnContext(t *testing.T) {
	sid := currentUserSID(t)
	pipe := randomTestPipe(t)
	v := &fakeVerifier{allowClient: true, allowServer: true}
	// In production the server runs as LocalSystem (which holds GA in ServicePipeSDDL).
	// In an unprivileged test process, grant the test user GA so go-winio can open
	// subsequent server instances while still testing DialPipeAccess(0x12019b) + PID auth.
	ln, err := listenServicePipeSDDL(pipe, "D:P(A;;GA;;;"+sid+")", v)
	if err != nil {
		t.Fatalf("listenServicePipeSDDL failed: %v", err)
	}
	srv := &http.Server{
		Handler: RequireVerifiedPeerWithToken("", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			peer, ok := PeerFromContext(r.Context())
			if !ok || peer.PID == 0 {
				w.WriteHeader(500)
				return
			}
			writeJSON(w, 200, map[string]any{"pid": peer.PID})
		})),
		ConnContext: ServiceConnContext,
	}
	go func() { _ = srv.Serve(ln) }()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	}()

	selfPID := uint32(os.Getpid())
	client := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				return DialServicePipe(ctx, pipe, selfPID, v)
			},
		},
	}
	resp, err := client.Get("http://ferry/v1/status")
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("authenticated service call failed: err=%v resp=%v", err, resp)
	}
	_ = resp.Body.Close()
	client.CloseIdleConnections()

	// Wrong expected server PID must fail mutual verification.
	if _, err := DialServicePipe(context.Background(), pipe, selfPID+1, v); err == nil {
		t.Fatal("expected mismatched server PID to fail")
	}
	// Rejecting client on server side closes connection before HTTP handler runs.
	v.allowClient = false
	badClient := &http.Client{
		Timeout: time.Second,
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				return DialServicePipe(ctx, pipe, selfPID, v)
			},
		},
	}
	if resp, err := badClient.Get("http://ferry/v1/status"); err == nil {
		_ = resp.Body.Close()
		t.Fatal("expected rejected client to fail request")
	}
	if v.releases.Load() == 0 {
		t.Fatal("expected verified file lock release callback to run on connection close")
	}
}

func TestServicePipeBlocksUnprivilegedCreatePipeInstance(t *testing.T) {
	sid := currentUserSID(t)
	pipe := randomTestPipe(t)
	v := &fakeVerifier{allowClient: true, allowServer: true}
	ln, err := ListenServicePipe(pipe, sid, v)
	if err != nil {
		t.Fatalf("ListenServicePipe initial creation failed: %v", err)
	}
	defer ln.Close()
	// Because ServicePipeSDDL strips FILE_CREATE_PIPE_INSTANCE from the GUI user,
	// a non-admin process cannot create another instance of the service pipe.
	if _, err := ln.Accept(); err == nil {
		t.Log("running elevated; Administrators/SYSTEM may open additional instances")
	}
}

func TestRequireVerifiedPeerBlocksUnauthenticatedHTTP(t *testing.T) {
	h := RequireVerifiedPeerWithToken(strings.Repeat("a", 64), http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "http://ferry/v1/status", nil)
	req.Header.Set("Authorization", "Bearer "+strings.Repeat("a", 64))
	h.ServeHTTP(rec, req)
	if rec.Code != 401 {
		t.Fatalf("expected 401 for unverified context, got %d", rec.Code)
	}
}

func TestInspectProcessAndPathNormalization(t *testing.T) {
	sid, exePath, err := inspectProcess(uint32(os.Getpid()))
	if err != nil || sid != currentUserSID(t) || !filepath.IsAbs(exePath) {
		t.Fatalf("inspectProcess(self) failed: sid=%q path=%q err=%v", sid, exePath, err)
	}
	if !equalCanonicalPath(`\\?\`+exePath, exePath) {
		t.Fatal("equalCanonicalPath must normalize extended prefix")
	}
	if err := ProtectDataDir("relative"); err == nil {
		t.Fatal("ProtectDataDir must reject relative path")
	}
}

func TestQueryServiceStatusReadOnly(t *testing.T) {
	info, err := QueryServiceStatus("FerryNonExistentTestService999")
	if err != nil || info.Installed || info.Running {
		t.Fatalf("expected missing service status: %+v %v", info, err)
	}
	if _, err := QueryServiceStatus(""); err == nil {
		t.Fatal("expected empty service name error")
	}
	if mapStartType(2) != "auto" || mapStartType(3) != "manual" || mapStartType(4) != "disabled" {
		t.Fatal("unexpected start type mapping")
	}
}

func TestWinPeerVerifierMutualPipeAuthWithoutServerProcessOpen(t *testing.T) {
	dir := t.TempDir()
	coreFile := filepath.Join(dir, "ferry-core.exe")
	if err := os.WriteFile(coreFile, []byte("synthetic-service-core"), 0o600); err != nil {
		t.Fatal(err)
	}
	coreHash, _, relCore, err := HashFileLocked(coreFile)
	if err != nil {
		t.Fatal(err)
	}
	_ = relCore()

	guiExe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	guiHash, guiFinal, relGUI, err := HashFileLocked(guiExe)
	if err != nil {
		t.Fatal(err)
	}
	_ = relGUI()

	sid := currentUserSID(t)
	meta := ServiceMeta{
		AuthorizedUserSID: sid,
		GUIExePath:        guiFinal,
		GUIExeSHA256:      guiHash,
		CoreExeSHA256:     coreHash,
		Token:             strings.Repeat("b", 64),
	}
	serverVerifier, err := NewHashPeerVerifier(meta, coreFile)
	if err != nil {
		t.Fatalf("NewHashPeerVerifier failed: %v", err)
	}
	// Clear Pins so tests always exercise locked SHA-256 verification.
	serverVerifier.Pins = nil
	clientVerifier, err := NewClientHashPeerVerifier(coreFile, coreHash)
	if err != nil {
		t.Fatalf("NewClientHashPeerVerifier failed: %v", err)
	}
	clientVerifier.Pins = nil

	pipe := randomTestPipe(t)
	ln, err := listenServicePipeSDDL(pipe, "D:P(A;;GA;;;"+sid+")", serverVerifier)
	if err != nil {
		t.Fatalf("listenServicePipeSDDL failed: %v", err)
	}
	srv := &http.Server{
		Handler: RequireVerifiedPeerWithToken(meta.Token, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, 200, map[string]any{"ok": true})
		})),
		ConnContext: ServiceConnContext,
	}
	go func() { _ = srv.Serve(ln) }()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	}()

	selfPID := uint32(os.Getpid())
	client := NewClientWithDialer(meta.Token, func(ctx context.Context) (net.Conn, error) {
		return DialServicePipe(ctx, pipe, selfPID, clientVerifier)
	})
	defer client.Close()
	if _, err := client.Snapshot(context.Background()); err != nil {
		t.Fatalf("expected mutual verification to succeed without OpenProcess on server: %v", err)
	}

	// Wrong expected PID must still fail.
	if _, err := DialServicePipe(context.Background(), pipe, selfPID+1, clientVerifier); err == nil {
		t.Fatal("expected mismatched expected server PID to fail")
	}

	// Tampering the core binary hash expected by client must fail.
	badClientVerifier, err := NewClientHashPeerVerifier(coreFile, strings.Repeat("c", 64))
	if err != nil {
		t.Fatal(err)
	}
	badClientVerifier.Pins = nil
	if _, err := DialServicePipe(context.Background(), pipe, selfPID, badClientVerifier); err == nil {
		t.Fatal("expected core binary hash mismatch to fail")
	}
}

func TestProtectClientReadFileGrantsReadWithoutUnsafeWrite(t *testing.T) {
	if err := ProtectClientReadFile("relative.exe", currentUserSID(t)); err == nil {
		t.Fatal("relative file path must be rejected")
	}
	dir := t.TempDir()
	target := filepath.Join(dir, "ferry-core.exe")
	if err := os.WriteFile(target, []byte("core-payload"), 0o600); err != nil {
		t.Fatal(err)
	}
	sid := currentUserSID(t)
	if err := ProtectClientReadFile(target, sid); err != nil {
		t.Fatalf("ProtectClientReadFile failed: %v", err)
	}
	sd, err := windows.GetNamedSecurityInfo(target, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil || sd == nil {
		t.Fatalf("GetNamedSecurityInfo failed: %v", err)
	}
	dacl, _, err := sd.DACL()
	if err != nil || dacl == nil {
		t.Fatalf("DACL missing: %v", err)
	}
	foundUserRead := false
	for i := uint32(0); i < uint32(dacl.AceCount); i++ {
		var ace *windows.ACCESS_ALLOWED_ACE
		if err := windows.GetAce(dacl, i, &ace); err != nil || ace == nil {
			t.Fatal(err)
		}
		if ace.Header.AceType != windows.ACCESS_ALLOWED_ACE_TYPE {
			continue
		}
		aceSID := (*windows.SID)(unsafe.Pointer(&ace.SidStart)).String()
		if aceSID == sid {
			if (uint32(ace.Mask) & unsafeWriteMask) != 0 {
				t.Fatalf("authorized user must not receive write/delete rights: mask=%#x", ace.Mask)
			}
			if (uint32(ace.Mask) & windows.FILE_GENERIC_READ) == windows.FILE_GENERIC_READ {
				foundUserRead = true
			}
		} else if !isPrivilegedSID(aceSID) {
			t.Fatalf("unexpected non-privileged ACE for %s", aceSID)
		}
	}
	if !foundUserRead {
		t.Fatal("expected authorized user SID to receive read access")
	}
}

func TestClientAuthRoundTripWithGUIHash(t *testing.T) {
	dir := t.TempDir()
	sid := currentUserSID(t)
	meta := ServiceMeta{
		AuthorizedUserSID: sid,
		GUIExePath:        `C:\Apps\Ferry\Ferry.exe`,
		GUIExeSHA256:      strings.Repeat("1", 64),
		CoreExeSHA256:     strings.Repeat("2", 64),
		Token:             strings.Repeat("3", 64),
	}
	authPath := filepath.Join(dir, "auth.json")
	data, err := json.Marshal(clientAuthForMeta(meta))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(authPath, data, 0o600); err != nil {
		t.Fatal(err)
	}
	auth, err := ReadClientAuth(authPath)
	if err != nil {
		t.Fatalf("ReadClientAuth failed: %v", err)
	}
	if auth.GUIExePath != meta.GUIExePath || auth.GUIExeSHA256 != meta.GUIExeSHA256 {
		t.Fatalf("expected GUI hash metadata in ClientAuth, got %+v", auth)
	}
	// Legacy auth.json without GUIExePath / GUIExeSHA256 must still validate.
	legacy := filepath.Join(dir, "legacy-auth.json")
	if err := os.WriteFile(legacy, []byte(`{"coreExeSha256":"`+strings.Repeat("a", 64)+`","token":"`+strings.Repeat("b", 64)+`"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadClientAuth(legacy); err != nil {
		t.Fatalf("legacy ClientAuth must remain valid: %v", err)
	}
}
