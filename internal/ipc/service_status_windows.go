//go:build windows

package ipc

import (
	"errors"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

// ServiceInfo reports the read-only SCM status of FerryCore.
type ServiceInfo struct {
	Installed bool   `json:"installed"`
	Running   bool   `json:"running"`
	StartType string `json:"startType"`
	Version   string `json:"version"`
	PID       uint32 `json:"-"`
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
	}
	return info, nil
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
