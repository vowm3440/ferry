//go:build windows

package ipc

import (
	"context"
	"crypto/subtle"
	"errors"
	"ferry/internal/spec"
	"ferry/internal/telemetry"
	"net"
	"os"
	"sync"
	"time"
)

var (
	selfPrintOnce   sync.Once
	selfGUIPath     string
	selfGUISHA256   string
	selfPrintLoaded bool
)

func currentGUIFingerprint() (string, string, bool) {
	selfPrintOnce.Do(func() {
		exe, err := os.Executable()
		if err != nil {
			return
		}
		digest, finalPath, release, err := HashFileLocked(exe)
		if err != nil {
			return
		}
		_ = release()
		selfGUIPath, selfGUISHA256, selfPrintLoaded = finalPath, digest, true
	})
	return selfGUIPath, selfGUISHA256, selfPrintLoaded
}

func authMatchesGUI(auth ClientAuth, guiPath, guiSHA256 string) bool {
	if auth.GUIExePath != "" && !equalCanonicalPath(auth.GUIExePath, guiPath) {
		return false
	}
	if auth.GUIExeSHA256 != "" && subtle.ConstantTimeCompare([]byte(auth.GUIExeSHA256), []byte(guiSHA256)) != 1 {
		return false
	}
	return true
}

// Router directs GUI IPC calls to the running FerryCore Windows service when
// available, and transparently falls back to the unprivileged child process.
type Router struct {
	mu          sync.Mutex
	process     *Process
	service     *Client
	svcPID      uint32
	svcTok      string
	svcActive   bool
	svcReplaced bool
	usingSvc    bool
	svcLogBase  int64
	procLogBase int64
	logHigh     int64
	badPID      uint32
	badUntil    time.Time
	wakeUntil   time.Time
	findService func() (ServiceInfo, InstallPaths, ClientAuth, bool)
	dialService func(pid uint32, paths InstallPaths, auth ClientAuth) (*Client, error)
}

func NewRouter(coreExecutable string) *Router {
	return &Router{process: NewProcess(coreExecutable)}
}

func (r *Router) serviceClient() *Client {
	find := r.findService
	if find == nil {
		find = r.loadMatchedService
	}
	info, paths, auth, ok := find()
	if !ok {
		r.dropServiceClient()
		return nil
	}
	if !info.Running && r.findService == nil {
		info, ok = r.tryWakeService(info)
		if !ok {
			r.dropServiceClient()
			return nil
		}
	}
	if cached, skip := r.cachedOrSkipped(info.PID, auth.Token); cached != nil || skip {
		return cached
	}
	return r.connectAndProbeService(info.PID, paths, auth)
}

func (r *Router) loadMatchedService() (ServiceInfo, InstallPaths, ClientAuth, bool) {
	info, err := QueryServiceStatus(ServiceName)
	if err != nil || !info.Installed {
		return ServiceInfo{}, InstallPaths{}, ClientAuth{}, false
	}
	paths, err := ServicePaths()
	if err != nil {
		return ServiceInfo{}, InstallPaths{}, ClientAuth{}, false
	}
	auth, err := ReadClientAuth(paths.ClientAuthPath)
	if err != nil || !ServiceBinaryMatches(info.BinaryPath, paths.ServiceCorePath) {
		return ServiceInfo{}, InstallPaths{}, ClientAuth{}, false
	}
	if guiPath, guiHash, loaded := currentGUIFingerprint(); loaded && !authMatchesGUI(auth, guiPath, guiHash) {
		return ServiceInfo{}, InstallPaths{}, ClientAuth{}, false
	}
	return info, paths, auth, true
}

func (r *Router) tryWakeService(info ServiceInfo) (ServiceInfo, bool) {
	if info.StartType == "disabled" {
		return info, false
	}
	r.mu.Lock()
	now := time.Now()
	allowWake := now.After(r.wakeUntil)
	if allowWake {
		r.wakeUntil = now.Add(5 * time.Second)
	}
	r.mu.Unlock()
	if !allowWake || StartWindowsService(ServiceName) != nil {
		return info, false
	}
	refreshed, err := QueryServiceStatus(ServiceName)
	if err != nil || !refreshed.Running || refreshed.PID == 0 {
		return refreshed, false
	}
	return refreshed, true
}

func (r *Router) cachedOrSkipped(pid uint32, token string) (*Client, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.service != nil && r.svcPID == pid && r.svcTok == token {
		return r.service, false
	}
	if pid == 0 || (pid == r.badPID && time.Now().Before(r.badUntil)) {
		return nil, true
	}
	return nil, false
}

func (r *Router) connectAndProbeService(pid uint32, paths InstallPaths, auth ClientAuth) *Client {
	dial := r.dialService
	if dial == nil {
		dial = dialAndVerifyService
	}
	client, err := dial(pid, paths, auth)
	if err != nil {
		r.markBadPID(pid)
		return nil
	}
	r.mu.Lock()
	if r.service != nil {
		r.service.Close()
	}
	if r.svcPID != 0 && r.svcPID != pid && r.svcActive {
		r.svcReplaced = true
	}
	if !r.usingSvc || r.svcPID != pid {
		r.svcLogBase = max(r.logHigh, r.process.HighSeq())
		r.usingSvc = true
	}
	r.service, r.svcPID, r.svcTok = client, pid, auth.Token
	r.badPID = 0
	r.mu.Unlock()
	_ = r.process.Close()
	return client
}

func (r *Router) markBadPID(pid uint32) {
	r.mu.Lock()
	r.badPID = pid
	r.badUntil = time.Now().Add(3 * time.Second)
	r.mu.Unlock()
}

func dialAndVerifyService(pid uint32, paths InstallPaths, auth ClientAuth) (*Client, error) {
	verifier, err := NewClientHashPeerVerifier(paths.ServiceCorePath, auth.CoreExeSHA256)
	if err != nil {
		return nil, err
	}
	client := NewClientWithDialer(auth.Token, func(ctx context.Context) (net.Conn, error) {
		return DialServicePipe(ctx, ServicePipeName, pid, verifier)
	})
	probeCtx, cancel := context.WithTimeout(context.Background(), 800*time.Millisecond)
	defer cancel()
	if _, err := client.Status(probeCtx); err != nil {
		client.Close()
		return nil, err
	}
	return client, nil
}

// InspectService checks whether FerryCore is installed, running, and healthy for guiExe.
func InspectService(guiExe string) (installed, running, needsRepair bool, startType string, err error) {
	info, err := QueryServiceStatus(ServiceName)
	if err != nil {
		return false, false, false, "disabled", err
	}
	if !info.Installed {
		return false, false, false, "disabled", nil
	}
	paths, pathErr := ServicePaths()
	auth, authErr := ReadClientAuth(paths.ClientAuthPath)
	if pathErr != nil || authErr != nil || !ServiceBinaryMatches(info.BinaryPath, paths.ServiceCorePath) {
		return true, info.Running, true, info.StartType, nil
	}
	if !guiMatchesAuth(guiExe, auth) {
		return true, info.Running, true, info.StartType, nil
	}
	if info.Running {
		client, dialErr := dialAndVerifyService(info.PID, paths, auth)
		if dialErr != nil {
			return true, true, true, info.StartType, nil
		}
		client.Close()
	}
	return true, info.Running, false, info.StartType, nil
}

func guiMatchesAuth(guiExe string, auth ClientAuth) bool {
	if auth.GUIExePath == "" && auth.GUIExeSHA256 == "" {
		return true
	}
	if guiPath, guiHash, loaded := currentGUIFingerprint(); loaded && equalCanonicalPath(guiPath, guiExe) {
		return authMatchesGUI(auth, guiPath, guiHash)
	}
	digest, finalPath, release, err := HashFileLocked(guiExe)
	if err != nil {
		return false
	}
	_ = release()
	return authMatchesGUI(auth, finalPath, digest)
}

func (r *Router) dropServiceClient() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.svcActive {
		r.svcReplaced = true
		r.svcActive = false
	}
	if r.usingSvc {
		r.procLogBase = r.logHigh
		r.usingSvc = false
	}
	if r.service != nil {
		r.service.Close()
		r.service = nil
		r.svcPID = 0
		r.svcTok = ""
	}
}

func (r *Router) StopUserProcess() error { return r.process.Close() }

func (r *Router) Status(ctx context.Context) (State, error) {
	if svc := r.serviceClient(); svc != nil {
		return svc.Status(ctx)
	}
	return r.process.Status(ctx)
}

func (r *Router) Snapshot(ctx context.Context) (Traffic, error) {
	if svc := r.serviceClient(); svc != nil {
		return svc.Snapshot(ctx)
	}
	return r.process.Snapshot(ctx)
}

func (r *Router) Connections(ctx context.Context) ([]telemetry.Connection, error) {
	if svc := r.serviceClient(); svc != nil {
		return svc.Connections(ctx)
	}
	return r.process.Connections(ctx)
}

func (r *Router) CloseConnection(ctx context.Context, id string) error {
	if svc := r.serviceClient(); svc != nil {
		return svc.CloseConnection(ctx, id)
	}
	return r.process.CloseConnection(ctx, id)
}

func (r *Router) SetLogLevel(ctx context.Context, level string) error {
	if svc := r.serviceClient(); svc != nil {
		return svc.SetLogLevel(ctx, level)
	}
	return r.process.SetLogLevel(ctx, level)
}

func (r *Router) TestRule(ctx context.Context, input telemetry.RuleTestRequest) (telemetry.RuleTestResult, error) {
	if svc := r.serviceClient(); svc != nil {
		return svc.TestRule(ctx, input)
	}
	return r.process.TestRule(ctx, input)
}

func (r *Router) RuleSetList(ctx context.Context) ([]telemetry.RuleSetInfo, error) {
	if svc := r.serviceClient(); svc != nil {
		return svc.RuleSetList(ctx)
	}
	return r.process.RuleSetList(ctx)
}

func (r *Router) RuleSetUpdate(ctx context.Context, id string) error {
	if svc := r.serviceClient(); svc != nil {
		return svc.RuleSetUpdate(ctx, id)
	}
	return r.process.RuleSetUpdate(ctx, id)
}

func (r *Router) RuleSetSync(ctx context.Context, sets []spec.RuleSet) error {
	if svc := r.serviceClient(); svc != nil {
		return svc.RuleSetSync(ctx, sets)
	}
	return r.process.RuleSetSync(ctx, sets)
}

func (r *Router) RuleSetDelete(ctx context.Context, id string) error {
	if svc := r.serviceClient(); svc != nil {
		return svc.RuleSetDelete(ctx, id)
	}
	return r.process.RuleSetDelete(ctx, id)
}

var _ = errors.New
