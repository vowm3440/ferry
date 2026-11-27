//go:build windows

package sysproxy

import (
	"errors"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const settingsKey = `Software\Microsoft\Windows\CurrentVersion\Internet Settings`

type windowsRegistry struct {
	keyPath    string
	skipNotify bool
}

func NewWindows(journal Journal) *Manager {
	return New(windowsRegistry{keyPath: settingsKey}, journal)
}

func (r windowsRegistry) targetKey() string {
	if r.keyPath == "" {
		return settingsKey
	}
	return r.keyPath
}

func (r windowsRegistry) Read() (Config, error) {
	var result Config
	key, err := registry.OpenKey(registry.CURRENT_USER, r.targetKey(), registry.QUERY_VALUE)
	if errors.Is(err, registry.ErrNotExist) {
		return result, nil
	}
	if err != nil {
		return result, err
	}
	defer key.Close()
	value, _, err := key.GetIntegerValue("ProxyEnable")
	if err == nil {
		result.Enabled = uint32(value)
		result.Present |= 1
	} else if !errors.Is(err, registry.ErrNotExist) {
		return result, err
	}
	for _, field := range []struct {
		name  string
		value *string
		flag  uint8
	}{{"ProxyServer", &result.Server, 2}, {"ProxyOverride", &result.Bypass, 4}, {"AutoConfigURL", &result.PAC, 8}} {
		value, _, err := key.GetStringValue(field.name)
		if err == nil {
			*field.value = value
			result.Present |= field.flag
		} else if !errors.Is(err, registry.ErrNotExist) {
			return result, err
		}
	}
	return result, nil
}

func (r windowsRegistry) Write(value Config) error {
	key, _, err := registry.CreateKey(registry.CURRENT_USER, r.targetKey(), registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer key.Close()
	if value.Present&1 != 0 {
		err = key.SetDWordValue("ProxyEnable", value.Enabled)
	} else {
		err = deleteValue(key, "ProxyEnable")
	}
	if err != nil {
		return err
	}
	for _, field := range []struct {
		name, value string
		flag        uint8
	}{{"ProxyServer", value.Server, 2}, {"ProxyOverride", value.Bypass, 4}, {"AutoConfigURL", value.PAC, 8}} {
		if value.Present&field.flag != 0 {
			err = key.SetStringValue(field.name, field.value)
		} else {
			err = deleteValue(key, field.name)
		}
		if err != nil {
			return err
		}
	}
	if r.skipNotify {
		return nil
	}
	return notify()
}

func deleteValue(key registry.Key, name string) error {
	err := key.DeleteValue(name)
	if errors.Is(err, registry.ErrNotExist) {
		return nil
	}
	return err
}
func notify() error {
	setOption := windows.NewLazySystemDLL("wininet.dll").NewProc("InternetSetOptionW")
	for _, option := range []uintptr{39, 37} {
		result, _, err := setOption.Call(0, option, 0, 0)
		if result == 0 {
			return err
		}
	}
	return nil
}
