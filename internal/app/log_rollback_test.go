package app

import (
	"context"
	"testing"
)

type levelRemote struct {
	inspectedRemote
	level string
}

func (r *levelRemote) SetLogLevel(_ context.Context, level string) error { r.level = level; return nil }

func TestLogLevelSaveFailureRestoresRunningLevel(t *testing.T) {
	s, fixture := configured(t)
	remote := &levelRemote{inspectedRemote: inspectedRemote{remoteFixture: *fixture}, level: "info"}
	s.remote = remote
	ctx := context.Background()
	if err := (&Core{s}).Start(ctx); err != nil {
		t.Fatal(err)
	}
	s.storage = failingStorage{}
	if err := s.Logs().SetLevel(ctx, "debug"); err == nil {
		t.Fatal("failed save accepted")
	}
	if remote.level != "info" || s.config.Log.Level != "info" {
		t.Fatal("failed save left runtime and configuration inconsistent")
	}
}
