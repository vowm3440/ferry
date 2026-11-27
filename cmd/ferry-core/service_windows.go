//go:build windows

package main

import (
	"errors"
	"ferry/internal/core/host"
	"ferry/internal/core/winsvc"
	"ferry/internal/ipc"
	"net/http"
	"os"
)

func runServiceCommand(command string, args []string) error {
	switch command {
	case "service":
		return winsvc.RunService(func(coreHost *host.Host) http.Handler {
			return ipc.NewServiceHandler(&adapter{core: coreHost})
		})
	case "uninstall-service":
		return winsvc.Uninstall()
	case "refresh-installed-service":
		coreExe, err := os.Executable()
		if err != nil {
			return errors.New("无法确定内核程序路径")
		}
		return winsvc.RefreshIfInstalled(coreExe)
	case "install-service", "repair-service":
		sid, guiExe, err := parseInstallFlags(args)
		if err != nil {
			return err
		}
		coreExe, err := os.Executable()
		if err != nil {
			return errors.New("无法确定内核程序路径")
		}
		if command == "repair-service" {
			return winsvc.Repair(sid, coreExe, guiExe)
		}
		return winsvc.Install(sid, coreExe, guiExe)
	default:
		return errors.New("不支持的服务子命令")
	}
}

func parseInstallFlags(args []string) (string, string, error) {
	var sid, guiExe string
	for i := 0; i+1 < len(args); i += 2 {
		switch args[i] {
		case "--user-sid":
			sid = args[i+1]
		case "--gui-exe":
			guiExe = args[i+1]
		}
	}
	if sid == "" || guiExe == "" {
		return "", "", errors.New("缺少服务安装参数（--user-sid 与 --gui-exe）")
	}
	return sid, guiExe, nil
}
