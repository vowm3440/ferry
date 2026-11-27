//go:build windows

package ipc

import (
	"context"
	"crypto/subtle"
	"errors"
	"ferry/internal/release"
	"ferry/internal/wintrust"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"unsafe"

	"github.com/Microsoft/go-winio"
	"golang.org/x/sys/windows"
)

var (
	kernel32DLL                 = windows.NewLazySystemDLL("kernel32.dll")
	procGetNamedPipeClientPID   = kernel32DLL.NewProc("GetNamedPipeClientProcessId")
	procGetNamedPipeServerPID   = kernel32DLL.NewProc("GetNamedPipeServerProcessId")
)

type peerContextKey struct{}

// PeerIdentity is attached to each authenticated service connection context.
type PeerIdentity struct {
	PID       uint32
	UserSID   string
	ImagePath string
	SignerPin string
}

// PeerVerifier validates the remote end of a service named-pipe connection and
// returns a release callback that keeps the verified image file locked until close.
type PeerVerifier interface {
	VerifyClient(conn net.Conn) (PeerIdentity, func() error, error)
	VerifyServer(conn net.Conn, expectedPID uint32) (PeerIdentity, func() error, error)
}

// WinPeerVerifier performs real Windows PID, SID, canonical path, and locked
// SHA-256 verification (plus Authenticode SPKI pinning when compiled in).
type WinPeerVerifier struct {
	AuthorizedUserSID  string
	ExpectedGUIPath    string
	ExpectedGUISHA256  string
	ExpectedCorePath   string
	ExpectedCoreSHA256 string
	Pins               []string
	VerifyFile         func(path string, pins []string) (*wintrust.VerifiedFile, error)
}

// NewHashPeerVerifier authenticates clients by Windows SID, canonical path,
// and locked-handle SHA-256 recorded at UAC installation time (no SPKI needed).
func NewHashPeerVerifier(meta ServiceMeta, corePath string) (*WinPeerVerifier, error) {
	if err := meta.Validate(); err != nil {
		return nil, err
	}
	return &WinPeerVerifier{
		AuthorizedUserSID:  meta.AuthorizedUserSID,
		ExpectedGUIPath:    filepath.Clean(meta.GUIExePath),
		ExpectedGUISHA256:  meta.GUIExeSHA256,
		ExpectedCorePath:   filepath.Clean(corePath),
		ExpectedCoreSHA256: meta.CoreExeSHA256,
		Pins:               release.SignerPins(),
		VerifyFile:         wintrust.Verify,
	}, nil
}

// NewClientHashPeerVerifier authenticates the FerryCore service process by SCM
// PID, canonical path in ProgramData, and locked-handle SHA-256.
func NewClientHashPeerVerifier(corePath, coreSHA256 string) (*WinPeerVerifier, error) {
	if !filepath.IsAbs(corePath) || !hex64Pattern.MatchString(coreSHA256) {
		return nil, errors.New("服务校验参数无效")
	}
	return &WinPeerVerifier{
		ExpectedCorePath:   filepath.Clean(corePath),
		ExpectedCoreSHA256: coreSHA256,
		Pins:               release.SignerPins(),
		VerifyFile:         wintrust.Verify,
	}, nil
}

func (v *WinPeerVerifier) VerifyClient(conn net.Conn) (PeerIdentity, func() error, error) {
	pid, err := pipePeerPID(conn, procGetNamedPipeClientPID)
	if err != nil {
		return PeerIdentity{}, nil, errors.New("无法识别客户端进程")
	}
	sid, imgPath, err := inspectProcess(pid)
	if err != nil {
		return PeerIdentity{}, nil, err
	}
	if !strings.EqualFold(sid, v.AuthorizedUserSID) {
		return PeerIdentity{}, nil, errors.New("客户端用户身份未获服务授权")
	}
	if !equalCanonicalPath(imgPath, v.ExpectedGUIPath) {
		return PeerIdentity{}, nil, errors.New("客户端程序路径与受信任安装目录不符")
	}
	return v.verifyImage(pid, sid, imgPath, v.ExpectedGUISHA256)
}

func (v *WinPeerVerifier) VerifyServer(conn net.Conn, expectedPID uint32) (PeerIdentity, func() error, error) {
	pid, err := pipePeerPID(conn, procGetNamedPipeServerPID)
	if err != nil || expectedPID == 0 || pid != expectedPID {
		return PeerIdentity{}, nil, errors.New("服务进程身份与系统服务管理器不一致")
	}
	if !filepath.IsAbs(v.ExpectedCorePath) {
		return PeerIdentity{}, nil, errors.New("服务程序路径无效")
	}
	// Do not call OpenProcess/OpenProcessToken on LocalSystem services from an
	// unprivileged GUI process; Windows denies access (ERROR_ACCESS_DENIED).
	// Kernel-reported GetNamedPipeServerProcessId == SCM PID + locked binary hash suffices.
	return v.verifyImage(pid, sidSystem, v.ExpectedCorePath, v.ExpectedCoreSHA256)
}

func (v *WinPeerVerifier) verifyImage(pid uint32, sid, imgPath, expectedSHA256 string) (PeerIdentity, func() error, error) {
	if len(v.Pins) > 0 {
		verify := v.VerifyFile
		if verify == nil {
			verify = wintrust.Verify
		}
		verified, err := verify(imgPath, v.Pins)
		if err != nil {
			return PeerIdentity{}, nil, errors.New("对端程序发行签名验证失败")
		}
		id := PeerIdentity{PID: pid, UserSID: sid, ImagePath: verified.FinalPath(), SignerPin: verified.SignerSPKI()}
		return id, verified.Close, nil
	}
	if expectedSHA256 == "" {
		return PeerIdentity{}, nil, errors.New("缺少对端程序完整性指纹")
	}
	digest, finalPath, release, err := HashFileLocked(imgPath)
	if err != nil {
		return PeerIdentity{}, nil, err
	}
	if subtle.ConstantTimeCompare([]byte(digest), []byte(expectedSHA256)) != 1 {
		_ = release()
		return PeerIdentity{}, nil, errors.New("对端程序完整性校验失败，请重新修复系统服务")
	}
	id := PeerIdentity{PID: pid, UserSID: sid, ImagePath: finalPath, SignerPin: "sha256:" + digest}
	return id, release, nil
}

func pipePeerPID(conn net.Conn, proc *windows.LazyProc) (uint32, error) {
	handle, err := connHandle(conn)
	if err != nil {
		return 0, err
	}
	var pid uint32
	r1, _, callErr := proc.Call(uintptr(handle), uintptr(unsafe.Pointer(&pid)))
	if r1 == 0 {
		return 0, callErr
	}
	return pid, nil
}

func connHandle(conn net.Conn) (windows.Handle, error) {
	type fdProvider interface{ Fd() uintptr }
	if provider, ok := conn.(fdProvider); ok {
		return windows.Handle(provider.Fd()), nil
	}
	if sc, ok := conn.(syscall.Conn); ok {
		raw, err := sc.SyscallConn()
		if err != nil {
			return 0, err
		}
		var h windows.Handle
		if err := raw.Control(func(fd uintptr) { h = windows.Handle(fd) }); err != nil {
			return 0, err
		}
		return h, nil
	}
	return 0, errors.New("不支持提取管道句柄")
}

func inspectProcess(pid uint32) (string, string, error) {
	proc, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return "", "", errors.New("无法打开对端进程进行身份核验")
	}
	defer windows.CloseHandle(proc)
	var token windows.Token
	if err := windows.OpenProcessToken(proc, windows.TOKEN_QUERY, &token); err != nil {
		return "", "", errors.New("无法读取对端进程令牌")
	}
	defer token.Close()
	user, err := token.GetTokenUser()
	if err != nil || user == nil || user.User.Sid == nil {
		return "", "", errors.New("无法读取对端用户 SID")
	}
	buf := make([]uint16, 1024)
	size := uint32(len(buf))
	if err := windows.QueryFullProcessImageName(proc, 0, &buf[0], &size); err != nil {
		return "", "", errors.New("无法读取对端进程可执行路径")
	}
	return user.User.Sid.String(), windows.UTF16ToString(buf[:size]), nil
}

func equalCanonicalPath(a, b string) bool {
	ca := strings.TrimPrefix(filepath.Clean(a), `\\?\`)
	cb := strings.TrimPrefix(filepath.Clean(b), `\\?\`)
	return strings.EqualFold(ca, cb)
}

type verifiedConn struct {
	net.Conn
	peer    PeerIdentity
	release func() error
	once    sync.Once
}

func (c *verifiedConn) Close() error {
	var releaseErr error
	c.once.Do(func() {
		if c.release != nil {
			releaseErr = c.release()
		}
	})
	return errors.Join(c.Conn.Close(), releaseErr)
}

type verifiedListener struct {
	net.Listener
	verifier PeerVerifier
}

func (l *verifiedListener) Accept() (net.Conn, error) {
	for {
		conn, err := l.Listener.Accept()
		if err != nil {
			return nil, err
		}
		peer, release, err := l.verifier.VerifyClient(conn)
		if err != nil {
			_ = conn.Close()
			continue
		}
		return &verifiedConn{Conn: conn, peer: peer, release: release}, nil
	}
}

// ListenServicePipe creates the fixed service pipe with a restricted DACL and
// wraps Accept to authenticate every client connection before HTTP handling.
func ListenServicePipe(pipeName, authorizedUserSID string, verifier PeerVerifier) (net.Listener, error) {
	sddl, err := ServicePipeSDDL(authorizedUserSID)
	if err != nil {
		return nil, err
	}
	return listenServicePipeSDDL(pipeName, sddl, verifier)
}

func listenServicePipeSDDL(pipeName, sddl string, verifier PeerVerifier) (net.Listener, error) {
	if pipeName == "" || verifier == nil {
		return nil, errors.New("服务管道配置无效")
	}
	raw, err := winio.ListenPipe(pipeName, &winio.PipeConfig{
		SecurityDescriptor: sddl, InputBufferSize: 65536, OutputBufferSize: 65536,
	})
	if err != nil {
		return nil, errors.New("无法创建受保护服务管道")
	}
	return &verifiedListener{Listener: raw, verifier: verifier}, nil
}

// DialServicePipe dials the service pipe using the restricted client access mask
// and verifies the server PID and binary signature before returning the connection.
func DialServicePipe(ctx context.Context, pipeName string, expectedPID uint32, verifier PeerVerifier) (net.Conn, error) {
	if pipeName == "" || verifier == nil {
		return nil, errors.New("服务管道连接参数无效")
	}
	conn, err := winio.DialPipeAccess(ctx, pipeName, ServicePipeAccessMask)
	if err != nil {
		return nil, errors.New("无法连接受保护系统服务")
	}
	peer, release, err := verifier.VerifyServer(conn, expectedPID)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	return &verifiedConn{Conn: conn, peer: peer, release: release}, nil
}

// PeerFromContext returns the verified peer attached by ServiceConnContext.
func PeerFromContext(ctx context.Context) (PeerIdentity, bool) {
	peer, ok := ctx.Value(peerContextKey{}).(PeerIdentity)
	return peer, ok
}

// ServiceConnContext attaches the verified connection's PeerIdentity to request contexts.
func ServiceConnContext(ctx context.Context, conn net.Conn) context.Context {
	if vc, ok := conn.(*verifiedConn); ok {
		return context.WithValue(ctx, peerContextKey{}, vc.peer)
	}
	return ctx
}

// RequireVerifiedPeerWithToken enforces both connection-level peer verification
// and (when token is non-empty) a 256-bit Bearer token installed at UAC time.
func RequireVerifiedPeerWithToken(token string, next http.Handler) http.Handler {
	slots := make(chan struct{}, 8)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := PeerFromContext(r.Context()); !ok {
			writeError(w, 401, "对端身份未通过服务认证")
			return
		}
		if token != "" {
			provided := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
			if len(token) != 64 || subtle.ConstantTimeCompare([]byte(provided), []byte(token)) != 1 {
				writeError(w, 401, "服务通信密钥未授权")
				return
			}
		}
		select {
		case slots <- struct{}{}:
			defer func() { <-slots }()
		default:
			writeError(w, 429, "内核忙，请稍后重试")
			return
		}
		if r.ContentLength > MaxBodySize {
			writeError(w, 413, "配置过大")
			return
		}
		next.ServeHTTP(w, r)
	})
}
