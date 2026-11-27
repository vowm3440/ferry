//go:build windows

// Package instance prevents another GUI from recovering an active proxy journal.
package instance

import (
	"errors"
	"sync"
	"time"

	"golang.org/x/sys/windows"
)

var ErrRunning = errors.New("Ferry 已在运行，请从系统托盘打开")

type Lock struct {
	mu     sync.Mutex
	handle windows.Handle
}

func Acquire() (*Lock, error) {
	name, err := userMutexName()
	if err != nil {
		return nil, err
	}
	return acquire(name)
}

// AcquireWithTimeout retries acquiring the per-user mutex until timeout so an
// elevated service-install restart can cleanly hand off from the exiting GUI.
func AcquireWithTimeout(timeout time.Duration) (*Lock, error) {
	name, err := userMutexName()
	if err != nil {
		return nil, err
	}
	return acquireWithTimeout(name, timeout)
}

func userMutexName() (string, error) {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return "", errors.New("无法确定当前用户")
	}
	return `Global\Ferry.User.` + user.User.Sid.String(), nil
}

func acquireWithTimeout(name string, timeout time.Duration) (*Lock, error) {
	deadline := time.Now().Add(timeout)
	for {
		lock, err := acquire(name)
		if err == nil || !errors.Is(err, ErrRunning) || !time.Now().Before(deadline) {
			return lock, err
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func acquire(name string) (*Lock, error) {
	encoded, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return nil, errors.New("实例锁名称无效")
	}
	handle, err := windows.CreateMutex(nil, false, encoded)
	if errors.Is(err, windows.ERROR_ALREADY_EXISTS) {
		if handle != 0 {
			if closeErr := windows.CloseHandle(handle); closeErr != nil {
				return nil, errors.New("无法关闭重复实例锁")
			}
		}
		return nil, ErrRunning
	}
	if err != nil {
		return nil, errors.New("无法创建用户实例锁")
	}
	return &Lock{handle: handle}, nil
}

func (l *Lock) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.handle == 0 {
		return nil
	}
	if err := windows.CloseHandle(l.handle); err != nil {
		return errors.New("无法释放用户实例锁")
	}
	l.handle = 0
	return nil
}
