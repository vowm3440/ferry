//go:build windows

package ipc

import (
	"errors"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

// ServiceInfo reports the read-only SCM status of FerryCore.
type ServiceInfo struct {
	Installed  bool   `json:"installed"`
	Running    bool   `json:"running"`
	StartType  string `json:"startType"`
	Version    string `json:"version"`
	PID        uint32 `json:"-"`
	BinaryPath string `json:"-"`
}

// QueryServiceStatus performs a read-only SCM query safe for unprivileged users.
func QueryServiceStatus(name string) (ServiceInfo, error) {
	info := ServiceInfo{StartType: "disabled"}
	if name == "" {
		return info, errors.New("服务名无效")
	}
	scm, err := windows.OpenSCManager(nil, nil, windows.SC_MANAGER_CONNECT)
	if err != nil {
		return info, errors.New("无法连接 Windows 服务管理器")
	}
	defer windows.CloseServiceHandle(scm)
	serviceName, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return info, errors.New("服务名无效")
	}
	handle, err := windows.OpenService(scm, serviceName, windows.SERVICE_QUERY_STATUS|windows.SERVICE_QUERY_CONFIG)
	if err != nil {
		if errors.Is(err, windows.ERROR_SERVICE_DOES_NOT_EXIST) {
			return info, nil
		}
		return info, errors.New("无法查询系统服务状态")
	}
	defer windows.CloseServiceHandle(handle)
	s := &mgr.Service{Name: name, Handle: handle}
	info.Installed = true
	if status, err := s.Query(); err == nil {
		info.Running = status.State == svc.Running
		info.PID = status.ProcessId
	}
	if cfg, err := s.Config(); err == nil {
		info.StartType = mapStartType(cfg.StartType)
		info.BinaryPath = cfg.BinaryPathName
	}
	return info, nil
}

// StartWindowsService starts an installed service without UAC if the authorized
// user has SERVICE_START access on the service object.
func StartWindowsService(name string) error {
	if name == "" {
		return errors.New("服务名无效")
	}
	scm, err := windows.OpenSCManager(nil, nil, windows.SC_MANAGER_CONNECT)
	if err != nil {
		return errors.New("无法连接 Windows 服务管理器")
	}
	defer windows.CloseServiceHandle(scm)
	serviceName, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return errors.New("服务名无效")
	}
	handle, err := windows.OpenService(scm, serviceName, windows.SERVICE_START|windows.SERVICE_QUERY_STATUS)
	if err != nil {
		return errors.New("无权或无法启动系统服务，请尝试修复服务")
	}
	defer windows.CloseServiceHandle(handle)
	s := &mgr.Service{Name: name, Handle: handle}
	if status, err := s.Query(); err == nil && status.State == svc.Running && status.ProcessId != 0 {
		return nil
	} else if err == nil && status.State != svc.StartPending {
		if err := s.Start(); err != nil {
			return errors.New("启动系统服务失败，请尝试修复服务")
		}
	}
	return waitServiceStarted(s, 8*time.Second)
}

func waitServiceStarted(s *mgr.Service, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		status, err := s.Query()
		if err == nil && status.State == svc.Running && status.ProcessId != 0 {
			return nil
		}
		time.Sleep(150 * time.Millisecond)
	}
	return errors.New("系统服务启动超时，请尝试修复服务")
}

// ServiceBinaryMatches verifies the SCM BinaryPathName executable equals expectedCorePath.
func ServiceBinaryMatches(binaryPath, expectedCorePath string) bool {
	if binaryPath == "" || !filepath.IsAbs(expectedCorePath) {
		return false
	}
	exe := extractServiceBinary(binaryPath)
	return exe != "" && equalCanonicalPath(exe, expectedCorePath)
}

func extractServiceBinary(cmdLine string) string {
	trimmed := strings.TrimSpace(cmdLine)
	if trimmed == "" {
		return ""
	}
	if strings.HasPrefix(trimmed, `"`) {
		if end := strings.IndexByte(trimmed[1:], '"'); end >= 0 {
			return trimmed[1 : 1+end]
		}
		return ""
	}
	lower := strings.ToLower(trimmed)
	if idx := strings.Index(lower, ".exe"); idx >= 0 {
		return trimmed[:idx+4]
	}
	fields := strings.Fields(trimmed)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

func mapStartType(startType uint32) string {
	switch startType {
	case mgr.StartAutomatic:
		return "auto"
	case mgr.StartManual:
		return "manual"
	default:
		return "disabled"
	}
}
