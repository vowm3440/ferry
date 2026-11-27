//go:build windows

package app

import (
	"errors"
	"path/filepath"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	shell32DLL         = windows.NewLazySystemDLL("shell32.dll")
	procShellExecuteW  = shell32DLL.NewProc("ShellExecuteW")
)

func launchInstallerExecutable(clean string) error {
	verb, _ := windows.UTF16PtrFromString("open")
	file, err := windows.UTF16PtrFromString(clean)
	if err != nil {
		return errors.New("安装程序路径无效")
	}
	params, _ := windows.UTF16PtrFromString("/S")
	dir, _ := windows.UTF16PtrFromString(filepath.Dir(clean))
	r1, _, callErr := procShellExecuteW.Call(
		0,
		uintptr(unsafe.Pointer(verb)),
		uintptr(unsafe.Pointer(file)),
		uintptr(unsafe.Pointer(params)),
		uintptr(unsafe.Pointer(dir)),
		1,
	)
	if r1 <= 32 {
		if callErr != nil && !errors.Is(callErr, windows.ERROR_SUCCESS) {
			return callErr
		}
		return errors.New("无法启动更新安装程序")
	}
	return nil
}
