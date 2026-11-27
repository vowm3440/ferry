package app

import (
	"context"
	"errors"
)

type ProxyController interface {
	Enable(int) error
	Disable() error
	Status() (bool, string, error)
}
type SysProxyStatus struct {
	Enabled bool     `json:"enabled"`
	Address string   `json:"address"`
	Bypass  []string `json:"bypass"`
}

func (s *Session) SetProxyController(proxy ProxyController) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.proxy = proxy
}

func (s *System) SetSysProxy(ctx context.Context, on bool) error {
	s.session.mu.Lock()
	defer s.session.mu.Unlock()
	if s.session.proxy == nil {
		return errors.New("系统代理控制器未初始化")
	}
	if !on {
		return s.session.proxy.Disable()
	}
	if s.session.state(ctx).Status != "running" {
		return errors.New("请先启动内核，再启用系统代理")
	}
	return s.session.proxy.Enable(int(s.session.config.Inbound.MixedPort))
}
func (s *System) SysProxyStatus(context.Context) (SysProxyStatus, error) {
	s.session.mu.Lock()
	defer s.session.mu.Unlock()
	if s.session.proxy == nil {
		return SysProxyStatus{}, errors.New("系统代理控制器未初始化")
	}
	on, address, err := s.session.proxy.Status()
	return SysProxyStatus{Enabled: on, Address: address, Bypass: []string{"<local>", "localhost", "127.*", "[::1]"}}, err
}

func (s *Session) RestoreSystemProxy() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.proxy != nil {
		return s.proxy.Disable()
	}
	return nil
}
