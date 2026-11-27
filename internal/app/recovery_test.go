package app

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

type crashedRemote struct {
	remoteFixture
	exited, fail bool
}

func (r *crashedRemote) Exited() bool { return r.exited }
func (r *crashedRemote) Start(ctx context.Context, data json.RawMessage) error {
	if r.fail {
		return errors.New("synthetic start failure")
	}
	r.exited = false
	return r.remoteFixture.Start(ctx, data)
}

func TestCoreCrashRecoversAndIntentionalStopDoesNotRestart(t *testing.T) {
	s, fixture := configured(t)
	remote := &crashedRemote{remoteFixture: *fixture}
	s.remote = remote
	ctx := context.Background()
	core := &Core{s}
	if err := core.Start(ctx); err != nil {
		t.Fatal(err)
	}
	proxy := &recoveryProxy{}
	s.SetProxyController(proxy)
	if err := s.System().SetSysProxy(ctx, true); err != nil {
		t.Fatal(err)
	}
	remote.exited = true
	remote.state.Status = "error"
	if err := s.RecoverCore(ctx, time.Now()); err != nil {
		t.Fatal(err)
	}
	if remote.starts != 2 || remote.exited || !proxy.enabled {
		t.Fatal("core crash not recovered")
	}
	if err := core.Stop(ctx); err != nil {
		t.Fatal(err)
	}
	remote.exited = true
	if err := s.RecoverCore(ctx, time.Now()); err != nil {
		t.Fatal(err)
	}
	if remote.starts != 2 {
		t.Fatal("intentional stop restarted core")
	}
}

func TestCrashRetryBudgetRestoresProxyAndStopsRetrying(t *testing.T) {
	s, fixture := configured(t)
	remote := &crashedRemote{remoteFixture: *fixture}
	s.remote = remote
	ctx := context.Background()
	if err := (&Core{s}).Start(ctx); err != nil {
		t.Fatal(err)
	}
	proxy := &recoveryProxy{enabled: true}
	s.SetProxyController(proxy)
	remote.fail = true
	remote.exited = true
	for range 3 {
		if err := s.RecoverCore(ctx, time.Now()); err == nil {
			t.Fatal("failed recovery hidden")
		}
	}
	if proxy.enabled {
		t.Fatal("dead proxy left enabled")
	}
	remote.fail = false
	if err := s.RecoverCore(ctx, time.Now()); err == nil {
		t.Fatal("retry budget not reported")
	}
	if remote.starts != 1 {
		t.Fatal("retried beyond limit")
	}
}
