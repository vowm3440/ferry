package app

import (
	"context"
	"errors"
	"ferry/internal/ipc"
	"slices"
)

type ServiceStatus struct {
	Installed bool   `json:"installed"`
	Running   bool   `json:"running"`
	StartType string `json:"startType"`
	Version   string `json:"version"`
}

type TunStatus struct {
	Enabled     bool     `json:"enabled"`
	Running     bool     `json:"running"`
	Stack       string   `json:"stack"`
	StrictRoute bool     `json:"strictRoute"`
	Excludes    []string `json:"excludes"`
	Error       string   `json:"error,omitempty"`
}

type ServiceController interface {
	Status() (ServiceStatus, error)
	Install() error
	Uninstall() error
	Repair() error
}

func (s *Session) SetServiceController(ctrl ServiceController) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.serviceCtrl = ctrl
}

func (s *System) ServiceStatus(context.Context) (ServiceStatus, error) {
	s.session.mu.Lock()
	ctrl := s.session.serviceCtrl
	s.session.mu.Unlock()
	if ctrl != nil {
		return ctrl.Status()
	}
	info, err := ipc.QueryServiceStatus(ipc.ServiceName)
	if err != nil {
		return ServiceStatus{StartType: "disabled"}, err
	}
	return ServiceStatus{
		Installed: info.Installed,
		Running:   info.Running,
		StartType: info.StartType,
		Version:   info.Version,
	}, nil
}

func (s *System) TunStatus(ctx context.Context) (TunStatus, error) {
	s.session.mu.Lock()
	tun := s.session.config.Inbound.Tun
	state := s.session.state(ctx)
	s.session.mu.Unlock()
	excludes := slices.Clone(tun.Excludes)
	if excludes == nil {
		excludes = []string{}
	}
	return TunStatus{
		Enabled:     tun.Enabled,
		Running:     tun.Enabled && state.Status == "running" && state.ServiceMode,
		Stack:       tun.Stack,
		StrictRoute: tun.StrictRoute,
		Excludes:    excludes,
	}, nil
}

func (s *System) InstallService(context.Context) error {
	ctrl, err := s.requireServiceController()
	if err != nil {
		return err
	}
	return ctrl.Install()
}

func (s *System) UninstallService(ctx context.Context) error {
	ctrl, err := s.requireServiceController()
	if err != nil {
		return err
	}
	_ = s.applyTun(ctx, false)
	return ctrl.Uninstall()
}

func (s *System) RepairService(context.Context) error {
	ctrl, err := s.requireServiceController()
	if err != nil {
		return err
	}
	return ctrl.Repair()
}

func (s *System) SetTun(ctx context.Context, on bool) error {
	if _, err := s.requireServiceController(); err != nil {
		return err
	}
	if !on {
		return s.applyTun(ctx, false)
	}
	status, err := s.ServiceStatus(ctx)
	if err != nil || !status.Installed || !status.Running {
		return errors.New("TUN 模式需要系统服务处于运行状态，请先在「设置 → TUN 与服务」中安装服务")
	}
	return s.applyTun(ctx, true)
}

func (s *System) requireServiceController() (ServiceController, error) {
	s.session.mu.Lock()
	ctrl := s.session.serviceCtrl
	s.session.mu.Unlock()
	if ctrl == nil {
		return nil, errors.New("当前运行环境不支持系统服务管理")
	}
	return ctrl, nil
}

func (s *System) applyTun(ctx context.Context, on bool) error {
	s.session.mu.Lock()
	defer s.session.mu.Unlock()
	next := s.session.snapshot()
	next.Config.Inbound.Tun.Enabled = on
	return s.session.reconfigure(ctx, next)
}
