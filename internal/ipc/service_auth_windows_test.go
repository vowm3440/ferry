//go:build windows

package ipc

import (
	"context"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestHashPeerVerifierAndTokenAuthWithoutSPKI(t *testing.T) {
	exePath, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	digest, finalPath, release, err := HashFileLocked(exePath)
	if err != nil {
		t.Fatalf("HashFileLocked failed: %v", err)
	}
	_ = release()

	sid := currentUserSID(t)
	token, err := GenerateServiceToken()
	if err != nil || len(token) != 64 {
		t.Fatalf("GenerateServiceToken failed: %q %v", token, err)
	}
	meta := ServiceMeta{
		AuthorizedUserSID: sid,
		GUIExePath:        finalPath,
		GUIExeSHA256:      digest,
		CoreExeSHA256:     digest,
		Token:             token,
	}
	serverVerifier, err := NewHashPeerVerifier(meta, finalPath)
	if err != nil {
		t.Fatalf("NewHashPeerVerifier failed: %v", err)
	}
	clientVerifier, err := NewClientHashPeerVerifier(finalPath, digest)
	if err != nil {
		t.Fatalf("NewClientHashPeerVerifier failed: %v", err)
	}

	pipe := randomTestPipe(t)
	ln, err := listenServicePipeSDDL(pipe, "D:P(A;;GA;;;"+sid+")", serverVerifier)
	if err != nil {
		t.Fatalf("listenServicePipeSDDL failed: %v", err)
	}
	srv := &http.Server{
		Handler: RequireVerifiedPeerWithToken(token, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			peer, ok := PeerFromContext(r.Context())
			if !ok || !strings.HasPrefix(peer.SignerPin, "sha256:") {
				w.WriteHeader(500)
				return
			}
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
	goodClient := NewClientWithDialer(token, func(ctx context.Context) (net.Conn, error) {
		return DialServicePipe(ctx, pipe, selfPID, clientVerifier)
	})
	defer goodClient.Close()
	if err := goodClient.call(context.Background(), "GET", "/v1/status", nil, nil); err != nil {
		t.Fatalf("SHA-256 + token authenticated call failed: %v", err)
	}

	// Wrong bearer token must be rejected with 401.
	badTokenClient := NewClientWithDialer(strings.Repeat("0", 64), func(ctx context.Context) (net.Conn, error) {
		return DialServicePipe(ctx, pipe, selfPID, clientVerifier)
	})
	defer badTokenClient.Close()
	if err := badTokenClient.call(context.Background(), "GET", "/v1/status", nil, nil); err == nil {
		t.Fatal("expected wrong bearer token to fail")
	}

	// Tampered/wrong expected SHA-256 must fail mutual client verification.
	badHashVerifier, _ := NewClientHashPeerVerifier(finalPath, strings.Repeat("f", 64))
	if _, err := DialServicePipe(context.Background(), pipe, selfPID, badHashVerifier); err == nil {
		t.Fatal("expected mismatched SHA-256 to fail verification")
	}
}

func TestServiceAuthFilesAndElevationValidation(t *testing.T) {
	if _, err := CurrentUserSID(); err != nil {
		t.Fatalf("CurrentUserSID failed: %v", err)
	}
	if err := RunElevated("relative\\ferry-core.exe", nil); err == nil {
		t.Fatal("relative executable must be rejected")
	}
	if err := RunElevated(`C:\Windows\System32\cmd.exe`, nil); err == nil {
		t.Fatal("non-ferry-core executable must be rejected")
	}
	if got := formatWinArgs([]string{"install-service", "--gui-exe", `C:\Program Files\Ferry\Ferry.exe`}); !strings.Contains(got, `"C:\Program Files\Ferry\Ferry.exe"`) {
		t.Fatalf("unexpected quoted args: %s", got)
	}

	dir := t.TempDir()
	metaPath := filepath.Join(dir, "service-meta.json")
	authPath := filepath.Join(dir, "auth.json")
	if _, err := ReadServiceMeta(metaPath); err == nil {
		t.Fatal("missing meta must fail")
	}
	if _, err := ReadClientAuth(authPath); err == nil {
		t.Fatal("missing auth must fail")
	}
	if err := os.WriteFile(authPath, []byte(`{"coreExeSha256":"bad","token":"bad"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadClientAuth(authPath); err == nil {
		t.Fatal("invalid auth must fail validation")
	}
}
