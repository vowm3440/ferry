package app

import (
	"context"
	"errors"
	"slices"
	"testing"
)

type fakeServiceController struct {
	installed bool
	running   bool
	failOp    bool
}

func (f *fakeServiceController) Status() (ServiceStatus, error) {
	st := "disabled"
	if f.installed {
		st = "auto"
	}
	return ServiceStatus{Installed: f.installed, Running: f.running, StartType: st, Version: "0.1.0-dev"}, nil
}
func (f *fakeServiceController) Install() error {
	if f.failOp {
		return errors.New("uac cancelled")
	}
	f.installed, f.running = true, true
	return nil
}
func (f *fakeServiceController) Uninstall() error {
	if f.failOp {
		return errors.New("uac cancelled")
	}
	f.installed, f.running = false, false
	return nil
}
func (f *fakeServiceController) Repair() error {
	return f.Install()
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
