package app

import (
	"context"
	"errors"
	"testing"

	"ferry/internal/ipc"
)

type rollbackRemote struct {
	remoteFixture
	selected string
}

func (r *rollbackRemote) SelectNode(_ context.Context, _, node string) error {
	r.selected = node
	return nil
}

type unavailableRemote struct{ remoteFixture }

func (r *unavailableRemote) Status(context.Context) (ipc.State, error) {
	return ipc.State{}, context.DeadlineExceeded
}

func TestReconfigureDoesNotCommitWhenStateUnknown(t *testing.T) {
	s, _ := configured(t)
	s.remote = &unavailableRemote{}
	next := s.snapshot()
	next.Settings.Theme = "light"
	if err := s.reconfigure(context.Background(), next); err == nil {
		t.Fatal("unknown running state committed")
	}
	if s.settings.Theme == "light" {
		t.Fatal("configuration changed on communication failure")
	}
}

func TestReconfigureFailureRestoresSelectedNode(t *testing.T) {
	s, fixture := configured(t)
	remote := &rollbackRemote{remoteFixture: *fixture}
	s.remote = remote
	ctx := context.Background()
	if err := (&Core{s}).Start(ctx); err != nil {
		t.Fatal(err)
	}
	if err := (&Core{s}).SelectNode(ctx, "proxy", "test"); err != nil {
		t.Fatal(err)
	}
	s.storage = failingStorage{}
	remote.selected = "reset-on-restart"
	err := s.reconfigure(ctx, s.snapshot())
	if err == nil {
		t.Fatal("failed storage accepted")
	}
	if remote.selected != "test" {
		t.Fatal("rollback did not restore selected outbound")
	}
	if errors.Is(err, context.Canceled) {
		t.Fatal("wrong failure")
	}
}
