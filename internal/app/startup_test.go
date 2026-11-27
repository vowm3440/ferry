package app

import (
	"context"
	"errors"
	"testing"
)

type recoveryProxy struct {
	recovered, enabled bool
	port               int
	err                error
}

func (p *recoveryProxy) Recover() error                { p.recovered = true; p.enabled = false; return p.err }
func (p *recoveryProxy) Enable(port int) error         { p.enabled = true; p.port = port; return nil }
func (p *recoveryProxy) Disable() error                { p.enabled = false; return nil }
func (p *recoveryProxy) Status() (bool, string, error) { return p.enabled, "127.0.0.1:7890", nil }

type corruptStorage struct{ proxy *recoveryProxy }

func (s corruptStorage) Load() ([]byte, error) {
	if !s.proxy.recovered {
		panic("business data read before proxy recovery")
	}
	return nil, errors.New("corrupt")
}
func (s corruptStorage) Save([]byte) error { return nil }

func TestProxyRecoveryRunsEvenWhenBusinessStateIsCorrupt(t *testing.T) {
	proxy := &recoveryProxy{enabled: true}
	if _, err := NewRecoveredSession(&remoteFixture{}, corruptStorage{proxy}, proxy); err == nil {
		t.Fatal("corrupt storage accepted")
	}
	if !proxy.recovered || proxy.enabled {
		t.Fatal("corrupt business state blocked network recovery")
	}
}

func TestSystemProxyRequiresRunningCoreAndRestoresOnStop(t *testing.T) {
	s, remote := configured(t)
	proxy := &recoveryProxy{}
	s.SetProxyController(proxy)
	ctx := context.Background()
	if err := s.System().SetSysProxy(ctx, true); err == nil || proxy.enabled {
		t.Fatal("enabled proxy without listener")
	}
	if err := (&Core{s}).Start(ctx); err != nil {
		t.Fatal(err)
	}
	if err := s.System().SetSysProxy(ctx, true); err != nil || proxy.port != 7890 {
		t.Fatal("wrong proxy port")
	}
	status, err := s.System().SysProxyStatus(ctx)
	if err != nil || !status.Enabled {
		t.Fatal("missing proxy status")
	}
	if err := (&Core{s}).Stop(ctx); err != nil || proxy.enabled || remote.state.Status != "stopped" {
		t.Fatal("stop left system proxy enabled")
	}
	if err := s.RestoreSystemProxy(); err != nil {
		t.Fatal(err)
	}
}
