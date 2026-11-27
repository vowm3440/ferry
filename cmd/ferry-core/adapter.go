package main

import (
	"context"
	"encoding/json"

	"ferry/internal/core/host"
	"ferry/internal/ipc"
	"ferry/internal/spec"
)

type adapter struct{ core *host.Host }

func (a *adapter) Status() ipc.State {
	s := a.core.Status()
	return ipc.State{Status: s.Status, Mode: s.Mode, UptimeMs: s.UptimeMs, MemoryBytes: s.MemoryBytes, Version: s.Version, CoreVersion: s.CoreVersion, ServiceMode: s.ServiceMode, Error: s.Error}
}

func (a *adapter) Start(ctx context.Context, raw json.RawMessage) error {
	s, err := spec.Decode(raw)
	if err != nil {
		return err
	}
	return a.core.Start(ctx, s)
}
func (a *adapter) Stop(ctx context.Context) error                 { return a.core.Stop(ctx) }
func (a *adapter) SetMode(ctx context.Context, mode string) error { return a.core.SetMode(ctx, mode) }
func (a *adapter) SelectNode(ctx context.Context, group, node string) error {
	return a.core.SelectNode(ctx, group, node)
}
func (a *adapter) Snapshot(ctx context.Context) (ipc.Traffic, error) {
	s, err := a.core.Snapshot(ctx)
	return ipc.Traffic{Up: s.Up, Down: s.Down, ActiveConns: s.ActiveConns, TotalUp: s.TotalUp, TotalDown: s.TotalDown, MemoryBytes: s.MemoryBytes, UpHistory: s.UpHistory, DownHistory: s.DownHistory}, err
}
