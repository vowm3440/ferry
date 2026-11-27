//go:build windows

package ipc

import (
	"context"
	"errors"
	"ferry/internal/telemetry"
)

func (p *Process) LatencyTest(ctx context.Context, ids []string) (map[string]int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.client == nil {
		return nil, errors.New("内核尚未启动")
	}
	return p.client.LatencyTest(ctx, ids)
}
func (p *Process) Connections(ctx context.Context) ([]telemetry.Connection, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.client == nil {
		return []telemetry.Connection{}, nil
	}
	return p.client.Connections(ctx)
}
func (p *Process) CloseConnection(ctx context.Context, id string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.client == nil {
		return errors.New("内核尚未启动")
	}
	return p.client.CloseConnection(ctx, id)
}
func (p *Process) Logs(ctx context.Context, after int64, limit int) ([]telemetry.LogEntry, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.client == nil {
		return []telemetry.LogEntry{}, nil
	}
	entries, err := p.client.Logs(ctx, max(0, after-p.logBase), limit)
	if err != nil {
		return nil, err
	}
	for i := range entries {
		entries[i].Seq += p.logBase
		p.logHigh = max(p.logHigh, entries[i].Seq)
	}
	return entries, nil
}
func (p *Process) SetLogLevel(ctx context.Context, level string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.client == nil {
		return errors.New("内核尚未启动")
	}
	return p.client.SetLogLevel(ctx, level)
}
