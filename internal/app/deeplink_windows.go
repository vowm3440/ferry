//go:build windows

package app

import (
	"context"
	"errors"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows/registry"
)

const defaultDeepLinkKeyPath = `Software\Classes\ferry`

type DeepLinkRegistrar interface {
	Registered() (bool, error)
	SetRegistered(enabled bool) error
}

type RegistryDeepLink struct {
	KeyPath    string
	Executable string
}

func NewWindowsDeepLinkRegistrar(executable string) *RegistryDeepLink {
	return &RegistryDeepLink{
		KeyPath:    defaultDeepLinkKeyPath,
		Executable: filepath.Clean(executable),
	}
}

func (r *RegistryDeepLink) Registered() (bool, error) {
	cmdKey := r.KeyPath + `\shell\open\command`
	key, err := registry.OpenKey(registry.CURRENT_USER, cmdKey, registry.QUERY_VALUE)
	if errors.Is(err, registry.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, errors.New("无法读取深链协议注册状态")
	}
	defer key.Close()
	val, _, err := key.GetStringValue("")
	if errors.Is(err, registry.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, errors.New("无法读取深链协议命令")
	}
	want := `"` + filepath.Clean(r.Executable) + `" "%1"`
	return strings.EqualFold(strings.TrimSpace(val), want), nil
}

func (r *RegistryDeepLink) SetRegistered(enabled bool) error {
	if !filepath.IsAbs(r.Executable) {
		return errors.New("应用路径无效，无法配置系统深链协议")
	}
	if !enabled {
		return deleteDeepLinkTree(r.KeyPath)
	}
	return writeDeepLinkTree(r.KeyPath, filepath.Clean(r.Executable))
}

func writeDeepLinkTree(root, exe string) error {
	base, _, err := registry.CreateKey(registry.CURRENT_USER, root, registry.SET_VALUE)
	if err != nil {
		return errors.New("无法创建深链协议注册表项")
	}
	defer base.Close()
	if base.SetStringValue("", "URL:Ferry Protocol") != nil || base.SetStringValue("URL Protocol", "") != nil {
		return errors.New("无法写入深链协议属性")
	}
	icon, _, err := registry.CreateKey(registry.CURRENT_USER, root+`\DefaultIcon`, registry.SET_VALUE)
	if err != nil {
		return errors.New("无法写入深链协议图标")
	}
	_ = icon.SetStringValue("", `"`+exe+`",0`)
	icon.Close()
	cmd, _, err := registry.CreateKey(registry.CURRENT_USER, root+`\shell\open\command`, registry.SET_VALUE)
	if err != nil {
		return errors.New("无法创建深链协议启动命令")
	}
	defer cmd.Close()
	if err := cmd.SetStringValue("", `"`+exe+`" "%1"`); err != nil {
		return errors.New("无法写入深链协议启动命令")
	}
	return nil
}

func deleteDeepLinkTree(root string) error {
	subkeys := []string{
		root + `\shell\open\command`,
		root + `\shell\open`,
		root + `\shell`,
		root + `\DefaultIcon`,
		root,
	}
	for _, k := range subkeys {
		if err := registry.DeleteKey(registry.CURRENT_USER, k); err != nil && !errors.Is(err, registry.ErrNotExist) {
			return errors.New("无法注销系统深链协议")
		}
	}
	return nil
}

func (s *Session) SetDeepLinkRegistrar(registrar DeepLinkRegistrar) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deepLinkReg = registrar
}

func (s *System) DeepLinkStatus(context.Context) (bool, error) {
	s.session.mu.Lock()
	reg := s.session.deepLinkReg
	s.session.mu.Unlock()
	if reg == nil {
		return false, nil
	}
	return reg.Registered()
}

func (s *System) SetDeepLinkRegistered(_ context.Context, on bool) error {
	s.session.mu.Lock()
	reg := s.session.deepLinkReg
	s.session.mu.Unlock()
	if reg == nil {
		return errors.New("当前运行环境不支持深链协议注册")
	}
	return reg.SetRegistered(on)
}
