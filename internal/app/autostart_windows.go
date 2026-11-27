//go:build windows

package app

import (
	"errors"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows/registry"
)

const runRegistryKey = `Software\Microsoft\Windows\CurrentVersion\Run`
const runValueName = "Ferry"

type AutoStartController interface {
	Set(enabled bool) error
	Enabled() (bool, error)
}

type RegistryAutoStart struct {
	KeyPath    string
	ValueName  string
	Executable string
}

func NewWindowsAutoStart(executable string) *RegistryAutoStart {
	return &RegistryAutoStart{
		KeyPath:    runRegistryKey,
		ValueName:  runValueName,
		Executable: filepath.Clean(executable),
	}
}

func (a *RegistryAutoStart) Set(enabled bool) error {
	if !filepath.IsAbs(a.Executable) {
		return errors.New("应用路径无效，无法配置开机自启")
	}
	key, _, err := registry.CreateKey(registry.CURRENT_USER, a.KeyPath, registry.SET_VALUE)
	if err != nil {
		return errors.New("无法访问开机自启注册表项")
	}
	defer key.Close()
	if !enabled {
		err = key.DeleteValue(a.ValueName)
		if errors.Is(err, registry.ErrNotExist) {
			return nil
		}
		if err != nil {
			return errors.New("无法关闭开机自启")
		}
		return nil
	}
	if err := key.SetStringValue(a.ValueName, `"`+a.Executable+`"`); err != nil {
		return errors.New("无法启用开机自启")
	}
	return nil
}

func (a *RegistryAutoStart) Enabled() (bool, error) {
	key, err := registry.OpenKey(registry.CURRENT_USER, a.KeyPath, registry.QUERY_VALUE)
	if errors.Is(err, registry.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, errors.New("无法读取开机自启状态")
	}
	defer key.Close()
	val, _, err := key.GetStringValue(a.ValueName)
	if errors.Is(err, registry.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, errors.New("无法读取开机自启状态")
	}
	cleaned := strings.Trim(strings.TrimSpace(val), `"`)
	return strings.EqualFold(filepath.Clean(cleaned), filepath.Clean(a.Executable)), nil
}

func (s *Session) SetAutoStartController(controller AutoStartController) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.autoStart = controller
}
