//go:build windows

// Package winsvc manages the privileged FerryCore Windows service lifecycle
// using Clash Verge Rev's UAC elevation + protected ProgramData + SHA-256/token
// IPC authentication model (no Authenticode SPKI certificate required).
package winsvc

import (
	"context"
	"errors"
	"ferry/internal/core/host"
	"ferry/internal/ipc"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

// Install cleanly replaces any previous FerryCore registration, copies ferry-core.exe
// into admin-protected ProgramData, records SHA-256 and a random 256-bit IPC secret,
// grants the authorized user non-elevated start permission, and starts FerryCore.
func Install(authorizedUserSID, sourceCorePath, guiExePath string) error {
	if _, err := ipc.ServicePipeSDDL(authorizedUserSID); err != nil {
		return err
	}
	if _, err := os.Stat(filepath.Clean(guiExePath)); err != nil {
		return errors.New("无法读取 Ferry 主程序进行完整性记录")
	}
	if err := stopAndDeleteExistingService(); err != nil {
		return err
	}
	paths, meta, err := stageServiceFiles(authorizedUserSID, sourceCorePath, guiExePath)
	if err != nil {
		return err
	}
	if err := ipc.WriteServiceAuthFiles(paths, meta); err != nil {
		return err
	}
	return registerAndStartService(paths.ServiceCorePath, authorizedUserSID)
}

func stageServiceFiles(authorizedUserSID, sourceCorePath, guiExePath string) (ipc.InstallPaths, ipc.ServiceMeta, error) {
	if _, err := ipc.ServicePipeSDDL(authorizedUserSID); err != nil {
		return ipc.InstallPaths{}, ipc.ServiceMeta{}, err
	}
	guiDigest, guiFinal, releaseGUI, err := ipc.HashFileLocked(guiExePath)
	if err != nil {
		return ipc.InstallPaths{}, ipc.ServiceMeta{}, errors.New("无法读取 Ferry 主程序进行完整性记录")
	}
	defer releaseGUI()
	paths, err := ipc.ServicePaths()
	if err != nil {
		return ipc.InstallPaths{}, ipc.ServiceMeta{}, err
	}
	for _, dir := range []string{paths.DataDir, paths.ServiceDir, paths.RuleSetsDir} {
		if err := ipc.ProtectDataDir(dir); err != nil {
			return ipc.InstallPaths{}, ipc.ServiceMeta{}, err
		}
	}
	if err := copyFileAtomic(sourceCorePath, paths.ServiceCorePath); err != nil {
		return ipc.InstallPaths{}, ipc.ServiceMeta{}, err
	}
	if err := ipc.ProtectClientReadFile(paths.ServiceCorePath, authorizedUserSID); err != nil {
		return ipc.InstallPaths{}, ipc.ServiceMeta{}, err
	}
	coreDigest, _, releaseCore, err := ipc.HashFileLocked(paths.ServiceCorePath)
	if err != nil {
		return ipc.InstallPaths{}, ipc.ServiceMeta{}, errors.New("无法校验受保护目录中的内核程序")
	}
	defer releaseCore()
	token, err := ipc.GenerateServiceToken()
	if err != nil {
		return ipc.InstallPaths{}, ipc.ServiceMeta{}, err
	}
	meta := ipc.ServiceMeta{
		AuthorizedUserSID: authorizedUserSID,
		GUIExePath:        guiFinal,
		GUIExeSHA256:      guiDigest,
		CoreExeSHA256:     coreDigest,
		Token:             token,
	}
	return paths, meta, nil
}

func copyFileAtomic(src, dst string) error {
	cleanSrc, cleanDst := filepath.Clean(src), filepath.Clean(dst)
	if !filepath.IsAbs(cleanSrc) || !filepath.IsAbs(cleanDst) {
		return errors.New("程序路径无效")
	}
	if cleanSrc == cleanDst {
		return nil
	}
	in, err := os.Open(cleanSrc)
	if err != nil {
		return errors.New("无法打开源内核程序")
	}
	defer in.Close()
	tmp := cleanDst + ".new"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o700)
	if err != nil {
		return errors.New("无法写入受保护内核副本")
	}
	if _, err := io.Copy(out, io.LimitReader(in, 256<<20)); err != nil {
		_ = out.Close()
		_ = os.Remove(tmp)
		return errors.New("复制内核程序失败")
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(tmp)
		return errors.New("写入内核副本失败")
	}
	return renameWithRetry(tmp, cleanDst)
}

func renameWithRetry(tmp, dst string) error {
	for attempt := 0; attempt < 10; attempt++ {
		_ = os.Remove(dst)
		if err := os.Rename(tmp, dst); err == nil {
			return nil
		}
		time.Sleep(150 * time.Millisecond)
	}
	_ = os.Remove(tmp)
	return errors.New("无法部署受保护内核程序")
}

func stopAndDeleteExistingService() error {
	m, err := mgr.Connect()
	if err != nil {
		return errors.New("无法连接 Windows 服务管理器（需要管理员权限）")
	}
	defer m.Disconnect()
	s, err := m.OpenService(ipc.ServiceName)
	if err != nil {
		if errors.Is(err, windows.ERROR_SERVICE_DOES_NOT_EXIST) {
			return nil
		}
		return errors.New("无法检查已有系统服务状态")
	}
	_, _ = s.Control(svc.Stop)
	waitServiceStopped(s, 8*time.Second)
	delErr := s.Delete()
	_ = s.Close()
	if delErr != nil && !errors.Is(delErr, windows.ERROR_SERVICE_MARKED_FOR_DELETE) {
		return errors.New("无法移除旧的系统服务注册项")
	}
	time.Sleep(300 * time.Millisecond)
	return nil
}

func registerAndStartService(serviceCorePath, authorizedUserSID string) error {
	m, err := mgr.Connect()
	if err != nil {
		return errors.New("无法连接 Windows 服务管理器（需要管理员权限）")
	}
	defer m.Disconnect()
	cfg := mgr.Config{
		StartType:   mgr.StartAutomatic,
		DisplayName: "Ferry Core Service",
		Description: "Ferry privileged sing-box TUN host service",
	}
	s, err := m.CreateService(ipc.ServiceName, serviceCorePath, cfg, "service")
	if err != nil {
		return errors.New("无法创建系统服务")
	}
	defer s.Close()
	configureServiceRecovery(s)
	_ = grantUserServiceStart(s.Handle, authorizedUserSID)
	if err := s.Start(); err != nil {
		return errors.New("系统服务已安装但启动失败")
	}
	return waitServiceRunning(s, 10*time.Second)
}

func configureServiceRecovery(s *mgr.Service) {
	actions := []mgr.RecoveryAction{
		{Type: mgr.ServiceRestart, Delay: 2 * time.Second},
		{Type: mgr.ServiceRestart, Delay: 5 * time.Second},
		{Type: mgr.ServiceRestart, Delay: 10 * time.Second},
	}
	if err := s.SetRecoveryActions(actions, 300); err == nil {
		_ = s.SetRecoveryActionsOnNonCrashFailures(true)
	}
}

func grantUserServiceStart(handle windows.Handle, authorizedUserSID string) error {
	if _, err := ipc.ServicePipeSDDL(authorizedUserSID); err != nil {
		return err
	}
	// Grant SYSTEM and Administrators SERVICE_ALL_ACCESS (0x000F01FF), and the
	// authorized GUI user READ_CONTROL|SERVICE_QUERY_CONFIG|SERVICE_QUERY_STATUS|SERVICE_START (0x00020095).
	sddl := "D:(A;;0x000F01FF;;;SY)(A;;0x000F01FF;;;BA)(A;;0x00020095;;;" + authorizedUserSID + ")"
	sd, err := windows.SecurityDescriptorFromString(sddl)
	if err != nil {
		return err
	}
	dacl, _, err := sd.DACL()
	if err != nil || dacl == nil {
		return errors.New("无法提取服务权限")
	}
	return windows.SetSecurityInfo(handle, windows.SE_SERVICE, windows.DACL_SECURITY_INFORMATION, nil, nil, dacl, nil)
}

// Uninstall stops and removes FerryCore and cleans up service auth artifacts.
func Uninstall() error {
	m, err := mgr.Connect()
	if err != nil {
		return errors.New("无法连接 Windows 服务管理器（需要管理员权限）")
	}
	defer m.Disconnect()
	if s, err := m.OpenService(ipc.ServiceName); err == nil {
		_, _ = s.Control(svc.Stop)
		waitServiceStopped(s, 8*time.Second)
		delErr := s.Delete()
		_ = s.Close()
		if delErr != nil && !errors.Is(delErr, windows.ERROR_SERVICE_MARKED_FOR_DELETE) {
			return errors.New("无法卸载系统服务")
		}
	} else if !errors.Is(err, windows.ERROR_SERVICE_DOES_NOT_EXIST) {
		return errors.New("无法打开系统服务")
	}
	if paths, err := ipc.ServicePaths(); err == nil {
		_ = os.Remove(paths.ServiceMetaPath)
		_ = os.Remove(paths.ClientAuthPath)
		_ = os.Remove(paths.ServiceCorePath)
	}
	return nil
}

// Repair cleanly reinstalls FerryCore with refreshed binary hashes and token.
func Repair(authorizedUserSID, sourceCorePath, guiExePath string) error {
	return Install(authorizedUserSID, sourceCorePath, guiExePath)
}

// RefreshIfInstalled updates an existing FerryCore installation after an
// in-place installer upgrade without disturbing users who never installed it.
func RefreshIfInstalled(sourceCorePath string) error {
	paths, err := ipc.ServicePaths()
	if err != nil {
		return err
	}
	return refreshIfInstalledAt(sourceCorePath, paths.ServiceMetaPath)
}

func refreshIfInstalledAt(sourceCorePath, metaPath string) error {
	clean := filepath.Clean(sourceCorePath)
	if !filepath.IsAbs(clean) {
		return errors.New("程序路径无效")
	}
	if _, err := os.Stat(clean); err != nil {
		return errors.New("无法打开源内核程序")
	}
	meta, err := ipc.ReadServiceMeta(metaPath)
	if err != nil {
		return nil
	}
	info, err := ipc.QueryServiceStatus(ipc.ServiceName)
	if err != nil || !info.Installed {
		return nil
	}
	guiExe := filepath.Join(filepath.Dir(clean), "Ferry.exe")
	if _, err := os.Stat(guiExe); err != nil {
		guiExe = meta.GUIExePath
	}
	return Install(meta.AuthorizedUserSID, clean, guiExe)
}

func waitServiceStopped(s *mgr.Service, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		status, err := s.Query()
		if err != nil || status.State == svc.Stopped {
			return
		}
		time.Sleep(150 * time.Millisecond)
	}
}

func waitServiceRunning(s *mgr.Service, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		status, err := s.Query()
		if err == nil && status.State == svc.Running {
			return nil
		}
		time.Sleep(150 * time.Millisecond)
	}
	return errors.New("系统服务启动超时")
}

// RunService executes FerryCore under the Windows Service Control Manager.
func RunService(buildHandler func(*host.Host) http.Handler) error {
	paths, err := ipc.ServicePaths()
	if err != nil {
		return err
	}
	if err := ipc.VerifyInstallTree(paths.ServiceDir); err != nil {
		return err
	}
	meta, err := ipc.ReadServiceMeta(paths.ServiceMetaPath)
	if err != nil {
		return err
	}
	verifier, err := ipc.NewHashPeerVerifier(meta, paths.ServiceCorePath)
	if err != nil {
		return err
	}
	return svc.Run(ipc.ServiceName, &serviceRunner{
		meta:         meta,
		paths:        paths,
		verifier:     verifier,
		buildHandler: buildHandler,
	})
}

type serviceRunner struct {
	meta         ipc.ServiceMeta
	paths        ipc.InstallPaths
	verifier     ipc.PeerVerifier
	buildHandler func(*host.Host) http.Handler
}

func (r *serviceRunner) Execute(_ []string, requests <-chan svc.ChangeRequest, status chan<- svc.Status) (bool, uint32) {
	const accepted = svc.AcceptStop | svc.AcceptShutdown
	status <- svc.Status{State: svc.StartPending}
	listener, err := ipc.ListenServicePipe(ipc.ServicePipeName, r.meta.AuthorizedUserSID, r.verifier)
	if err != nil {
		return false, 1
	}
	coreHost := host.NewService(r.paths.RuleSetsDir)
	server := &http.Server{
		Handler:           ipc.RequireVerifiedPeerWithToken(r.meta.Token, r.buildHandler(coreHost)),
		ConnContext:       ipc.ServiceConnContext,
		ReadHeaderTimeout: 3 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      20 * time.Second,
		IdleTimeout:       30 * time.Second,
		MaxHeaderBytes:    8 << 10,
	}
	errCh := make(chan error, 1)
	go func() { errCh <- server.Serve(listener) }()
	status <- svc.Status{State: svc.Running, Accepts: accepted}
	r.waitControl(requests, status, errCh)
	status <- svc.Status{State: svc.StopPending}
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer shutdownCancel()
	_ = server.Shutdown(shutdownCtx)
	_ = coreHost.Stop(shutdownCtx)
	return false, 0
}

func (r *serviceRunner) waitControl(requests <-chan svc.ChangeRequest, status chan<- svc.Status, errCh <-chan error) {
	for {
		select {
		case <-errCh:
			return
		case req := <-requests:
			switch req.Cmd {
			case svc.Interrogate:
				status <- req.CurrentStatus
			case svc.Stop, svc.Shutdown:
				return
			}
		}
	}
}
