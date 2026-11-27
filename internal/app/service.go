package app

import (
	"context"
	"errors"
	"slices"
)

type ServiceStatus struct {
	Installed   bool   `json:"installed"`
	Running     bool   `json:"running"`
	NeedsRepair bool   `json:"needsRepair,omitempty"`
	StartType   string `json:"startType"`
	Version     string `json:"version"`
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

type ServiceWakeController interface {
	EnsureRunning() error
}

func (s *Session) SetServiceController(ctrl ServiceController) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.serviceCtrl = ctrl
}

func (s *Session) SetServiceRestarter(fn func(startCore, restoreProxy bool) error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.serviceRestarter = fn
}

func (s *Session) AutoStartAfterServiceInstall(ctx context.Context, restoreProxy bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.config.Nodes) == 0 {
		return nil
	}
	if err := (&Core{s}).start(ctx); err != nil {
		return err
	}
	s.recovery.desired = true
	s.recovery.attempts = nil
	if (restoreProxy || s.settings.SysProxyOnStart) && s.proxy != nil {
		if err := s.proxy.Enable(int(s.config.Inbound.MixedPort)); err != nil {
			return err
		}
		s.recovery.proxyWanted = true
	}
	return nil
}

func (s *System) ServiceStatus(context.Context) (ServiceStatus, error) {
	s.session.mu.Lock()
	ctrl := s.session.serviceCtrl
	s.session.mu.Unlock()
	if ctrl == nil {
		return ServiceStatus{StartType: "disabled"}, nil
	}
	return ctrl.Status()
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

func (s *System) InstallService(ctx context.Context) error {
	ctrl, err := s.requireServiceController()
	if err != nil {
		return err
	}
	return s.runServiceSetup(ctx, ctrl.Install)
}

func (s *System) UninstallService(ctx context.Context) error {
	ctrl, err := s.requireServiceController()
	if err != nil {
		return err
	}
	_ = s.applyTun(ctx, false)
	return ctrl.Uninstall()
}

func (s *System) RepairService(ctx context.Context) error {
	ctrl, err := s.requireServiceController()
	if err != nil {
		return err
	}
	return s.runServiceSetup(ctx, ctrl.Repair)
}

func (s *System) runServiceSetup(ctx context.Context, op func() error) error {
	hasNodes, wasRunning, proxyWasOn, restarter, err := s.session.prepareServiceSetup(ctx)
	if err != nil {
		return err
	}
	if err := op(); err != nil {
		return err
	}
	if restarter != nil {
		return restarter(hasNodes || wasRunning, proxyWasOn)
	}
	if wasRunning && hasNodes {
		return s.session.AutoStartAfterServiceInstall(ctx, proxyWasOn)
	}
	return nil
}

func (s *Session) prepareServiceSetup(ctx context.Context) (hasNodes, wasRunning, proxyWasOn bool, restarter func(bool, bool) error, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	hasNodes = len(s.config.Nodes) > 0
	wasRunning = s.state(ctx).Status == "running"
	restarter = s.serviceRestarter
	if s.proxy != nil {
		proxyWasOn, _, _ = s.proxy.Status()
	}
	s.recovery.desired = false
	s.recovery.proxyWanted = false
	if wasRunning {
		if stopErr := s.remote.Stop(ctx); stopErr != nil {
			return false, false, false, nil, stopErr
		}
	}
	if proxyWasOn && s.proxy != nil {
		_ = s.proxy.Disable()
	}
	return hasNodes, wasRunning, proxyWasOn, restarter, nil
}

func (s *System) SetTun(ctx context.Context, on bool) error {
	ctrl, err := s.requireServiceController()
	if err != nil {
		return err
	}
	if !on {
		return s.applyTun(ctx, false)
	}
	status, err := s.ServiceStatus(ctx)
	if err != nil || !status.Installed {
		return errors.New("TUN 模式需要安装系统服务，请先安装服务")
	}
	if !status.Running || status.NeedsRepair {
		if wake, ok := ctrl.(ServiceWakeController); ok && wake.EnsureRunning() == nil {
			status, err = s.ServiceStatus(ctx)
		}
	}
	if err != nil || !status.Running || status.NeedsRepair {
		return errors.New("系统服务未运行或需要更新，请先启动或修复系统服务")
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
