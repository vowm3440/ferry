package app

import (
	"context"
	"errors"
	"slices"
	"testing"
)

type fakeServiceController struct {
	installed   bool
	running     bool
	needsRepair bool
	failOp      bool
	failWake    bool
	wakeCalls   int
}

func (f *fakeServiceController) Status() (ServiceStatus, error) {
	st := "disabled"
	if f.installed {
		st = "auto"
	}
	return ServiceStatus{
		Installed:   f.installed,
		Running:     f.running,
		NeedsRepair: f.needsRepair,
		StartType:   st,
		Version:     "0.1.0-dev",
	}, nil
}
func (f *fakeServiceController) Install() error {
	if f.failOp {
		return errors.New("uac cancelled")
	}
	f.installed, f.running, f.needsRepair = true, true, false
	return nil
}
func (f *fakeServiceController) Uninstall() error {
	if f.failOp {
		return errors.New("uac cancelled")
	}
	f.installed, f.running, f.needsRepair = false, false, false
	return nil
}
func (f *fakeServiceController) Repair() error {
	return f.Install()
}
func (f *fakeServiceController) EnsureRunning() error {
	f.wakeCalls++
	if f.failWake || !f.installed || f.needsRepair {
		return errors.New("服务未运行，请修复服务")
	}
	f.running = true
	return nil
}

func TestServiceAndTunLifecycleWithoutSPKI(t *testing.T) {
	ctx := context.Background()
	s, _ := configured(t)
	system := s.System()

	// Without a service controller attached, privileged capabilities stay hidden.
	if slices.Contains(system.Capabilities(ctx), "system.tun") || slices.Contains(system.Capabilities(ctx), "system.service") {
		t.Fatal("session without service controller must not advertise service/tun")
	}
	if err := system.InstallService(ctx); err == nil {
		t.Fatal("expected missing service controller error")
	}

	ctrl := &fakeServiceController{}
	s.SetServiceController(ctrl)
	caps := system.Capabilities(ctx)
	if !slices.Contains(caps, "system.service") || !slices.Contains(caps, "system.tun") {
		t.Fatalf("expected system.service and system.tun capabilities: %v", caps)
	}
	// Enabling TUN before service is installed must fail cleanly.
	if err := system.SetTun(ctx, true); err == nil {
		t.Fatal("expected TUN enable to require installed running service")
	}
	if err := system.InstallService(ctx); err != nil {
		t.Fatalf("InstallService failed: %v", err)
	}
	st, err := system.ServiceStatus(ctx)
	if err != nil || !st.Installed || !st.Running {
		t.Fatalf("unexpected service status after install: %+v %v", st, err)
	}
	if err := system.SetTun(ctx, true); err != nil {
		t.Fatalf("SetTun(true) failed with running service: %v", err)
	}
	tun, err := system.TunStatus(ctx)
	if err != nil || !tun.Enabled {
		t.Fatalf("expected TUN enabled: %+v %v", tun, err)
	}
	if err := system.UninstallService(ctx); err != nil {
		t.Fatalf("UninstallService failed: %v", err)
	}
	tun, _ = system.TunStatus(ctx)
	if tun.Enabled {
		t.Fatal("UninstallService must turn off TUN")
	}
	if err := system.RepairService(ctx); err != nil {
		t.Fatalf("RepairService failed: %v", err)
	}
}

func TestSetTunWakesStoppedServiceAndReportsRepairAccurately(t *testing.T) {
	ctx := context.Background()
	s, _ := configured(t)
	system := s.System()
	ctrl := &fakeServiceController{installed: true, running: false}
	s.SetServiceController(ctrl)

	if err := system.SetTun(ctx, true); err != nil {
		t.Fatalf("expected SetTun(true) to wake stopped service: %v", err)
	}
	if ctrl.wakeCalls != 1 || !ctrl.running {
		t.Fatalf("expected EnsureRunning to be called once, wakeCalls=%d running=%v", ctrl.wakeCalls, ctrl.running)
	}
	if err := system.SetTun(ctx, false); err != nil {
		t.Fatalf("SetTun(false) failed: %v", err)
	}

	// If installed service needs repair, SetTun must NOT tell the user to install service first.
	ctrl.running = false
	ctrl.needsRepair = true
	err := system.SetTun(ctx, true)
	if err == nil {
		t.Fatal("expected SetTun(true) to fail when service needs repair")
	}
	if err.Error() == "TUN 模式需要系统服务处于运行状态，请先在「设置 → TUN 与服务」中安装服务" {
		t.Fatalf("SetTun gave misleading install prompt for already-installed service: %v", err)
	}
}

type fakeProxyForService struct {
	enabled bool
	port    int
}

func (p *fakeProxyForService) Status() (bool, string, error) { return p.enabled, "127.0.0.1:7890", nil }
func (p *fakeProxyForService) Enable(port int) error        { p.enabled, p.port = true, port; return nil }
func (p *fakeProxyForService) Disable() error               { p.enabled = false; return nil }

func TestInstallServiceStopsCoreTriggersRestartAndAutoStarts(t *testing.T) {
	ctx := context.Background()
	s, remote := configured(t)
	proxy := &fakeProxyForService{}
	s.SetProxyController(proxy)
	ctrl := &fakeServiceController{}
	s.SetServiceController(ctrl)

	core := &Core{s}
	if err := core.Start(ctx); err != nil {
		t.Fatal(err)
	}
	if err := s.System().SetSysProxy(ctx, true); err != nil {
		t.Fatal(err)
	}

	var restartCalled bool
	var restartStartCore, restartRestoreProxy bool
	s.SetServiceRestarter(func(startCore, restoreProxy bool) error {
		restartCalled = true
		restartStartCore = startCore
		restartRestoreProxy = restoreProxy
		return nil
	})
	if err := s.System().InstallService(ctx); err != nil {
		t.Fatalf("InstallService failed: %v", err)
	}
	if remote.state.Status != "stopped" {
		t.Fatalf("expected old core to be stopped before service install, got %s", remote.state.Status)
	}
	if !restartCalled || !restartStartCore || !restartRestoreProxy {
		t.Fatalf("expected restarter(true, true), got called=%v start=%v proxy=%v", restartCalled, restartStartCore, restartRestoreProxy)
	}

	// Simulate restarted Ferry instance auto-starting the core on the newly installed service.
	if err := s.AutoStartAfterServiceInstall(ctx, true); err != nil {
		t.Fatalf("AutoStartAfterServiceInstall failed: %v", err)
	}
	if remote.state.Status != "running" || !proxy.enabled {
		t.Fatalf("expected core running and proxy restored after restart, status=%s proxy=%v", remote.state.Status, proxy.enabled)
	}

	// Empty session without nodes must not fail AutoStartAfterServiceInstall.
	empty, _ := emptySession()
	if err := empty.AutoStartAfterServiceInstall(ctx, false); err != nil {
		t.Fatalf("AutoStartAfterServiceInstall on empty session failed: %v", err)
	}
}
