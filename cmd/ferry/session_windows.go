//go:build windows

package main

import (
	"errors"
	"os"
	"path/filepath"
	"unsafe"

	"ferry/internal/app"
	"ferry/internal/instance"
	"ferry/internal/store"
	"ferry/internal/sysproxy"

	"golang.org/x/sys/windows"
)

func loadSession(remote app.Remote) (*app.Session, error) {
	directory := os.Getenv("FERRY_CONFIG_DIR")
	if directory == "" {
		local := os.Getenv("LOCALAPPDATA")
		if local == "" {
			return nil, errors.New("无法确定用户配置目录")
		}
		directory = filepath.Join(local, "Ferry")
	}
	if !filepath.IsAbs(directory) {
		return nil, errors.New("配置目录须为绝对路径")
	}
	proxy := sysproxy.NewWindows(store.New(filepath.Join(directory, "proxy-restore.dat")))
	session, err := app.NewRecoveredSession(remote, store.New(filepath.Join(directory, "state.dat")), proxy)
	if err != nil {
		return nil, err
	}
	session.SetDataDirs(directory, filepath.Join(directory, "logs"), nil)
	if exe, err := os.Executable(); err == nil {
		session.SetAutoStartController(app.NewWindowsAutoStart(exe))
		session.SetDeepLinkRegistrar(app.NewWindowsDeepLinkRegistrar(exe))
	}
	return session, nil
}

func startupError(err error) {
	titleText := "Ferry — 无法加载配置"
	flags := uintptr(0x10)
	if errors.Is(err, instance.ErrRunning) {
		titleText = "Ferry"
		flags = 0x40
	}
	message, _ := windows.UTF16PtrFromString(err.Error())
	title, _ := windows.UTF16PtrFromString(titleText)
	messageBox := windows.NewLazySystemDLL("user32.dll").NewProc("MessageBoxW")
	messageBox.Call(0, uintptr(unsafe.Pointer(message)), uintptr(unsafe.Pointer(title)), flags)
}
