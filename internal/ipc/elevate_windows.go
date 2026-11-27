//go:build windows

package ipc

import (
	"errors"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	seeMaskNoCloseProcess = 0x00000040
	seeMaskNoAsync        = 0x00000100
	swHide                = 0
)

var (
	shell32DLL         = windows.NewLazySystemDLL("shell32.dll")
	procShellExecuteEx = shell32DLL.NewProc("ShellExecuteExW")
)

type shellExecuteInfo struct {
	cbSize         uint32
	fMask          uint32
	hwnd           uintptr
	lpVerb         *uint16
	lpFile         *uint16
	lpParameters   *uint16
	lpDirectory    *uint16
	nShow          int32
	hInstApp       uintptr
	lpIDList       uintptr
	lpClass        *uint16
	hkeyClass      uintptr
	dwHotKey       uint32
	hIconOrMonitor uintptr
	hProcess       windows.Handle
}

func CurrentUserSID() (string, error) {
	u, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil || u == nil || u.User.Sid == nil {
		return "", errors.New("无法获取当前 Windows 用户 SID")
	}
	return parseUserSID(u.User.Sid.String())
}

// RunElevated launches executable with UAC ("runas") like Clash Verge Rev and
// waits for the elevated helper process to complete.
func RunElevated(executable string, args []string) error {
	clean := filepath.Clean(executable)
	if !filepath.IsAbs(clean) || filepath.Base(clean) != "ferry-core.exe" {
		return errors.New("服务安装程序路径无效")
	}
	verb, _ := windows.UTF16PtrFromString("runas")
	file, err := windows.UTF16PtrFromString(clean)
	if err != nil {
		return errors.New("程序路径无效")
	}
	params, err := windows.UTF16PtrFromString(formatWinArgs(args))
	if err != nil {
		return errors.New("安装参数无效")
	}
	dir, _ := windows.UTF16PtrFromString(filepath.Dir(clean))
	info := shellExecuteInfo{
		cbSize:       uint32(unsafe.Sizeof(shellExecuteInfo{})),
		fMask:        seeMaskNoCloseProcess | seeMaskNoAsync,
		lpVerb:       verb,
		lpFile:       file,
		lpParameters: params,
		lpDirectory:  dir,
		nShow:        swHide,
	}
	r1, _, callErr := procShellExecuteEx.Call(uintptr(unsafe.Pointer(&info)))
	if r1 == 0 {
		if errors.Is(callErr, windows.ERROR_CANCELLED) {
			return errors.New("已取消管理员权限确认")
		}
		return errors.New("无法请求管理员权限（UAC）")
	}
	if info.hProcess == 0 {
		return errors.New("无法等待提权操作完成")
	}
	defer windows.CloseHandle(info.hProcess)
	return waitElevatedExit(info.hProcess)
}

func waitElevatedExit(handle windows.Handle) error {
	event, err := windows.WaitForSingleObject(handle, 60_000)
	if err != nil || event != windows.WAIT_OBJECT_0 {
		return errors.New("等待提权操作超时")
	}
	var code uint32
	if err := windows.GetExitCodeProcess(handle, &code); err != nil {
		return errors.New("无法读取提权操作结果")
	}
	if code != 0 {
		return errors.New("系统服务操作失败，请检查管理员权限或杀毒软件拦截")
	}
	return nil
}

func formatWinArgs(args []string) string {
	quoted := make([]string, len(args))
	for i, arg := range args {
		quoted[i] = syscall.EscapeArg(arg)
	}
	return strings.Join(quoted, " ")
}
