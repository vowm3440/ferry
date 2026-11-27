//go:build windows

package ipc

import (
	"context"
	"encoding/json"
	"errors"
	"ferry/internal/telemetry"
	"time"
)

func (r *Router) Close() error {
	var stopErr error
	r.mu.Lock()
	svc := r.service
	active := r.svcActive
	r.svcActive = false
	r.svcReplaced = false
	r.mu.Unlock()
	if svc != nil && active {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		stopErr = svc.Stop(ctx)
		cancel()
	}
	r.dropServiceClient()
	return errors.Join(stopErr, r.process.Close())
}

func (r *Router) Exited() bool {
	svc := r.serviceClient()
	if r.consumeServiceReplaced() {
		return true
	}
	if svc != nil {
		return false
	}
	return r.process.Exited()
}

func (r *Router) consumeServiceReplaced() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	replaced := r.svcReplaced
	r.svcReplaced = false
	return replaced
}

func (r *Router) Start(ctx context.Context, config json.RawMessage) error {
	if svc := r.serviceClient(); svc != nil {
		if err := svc.Start(ctx, config); err != nil {
			return err
		}
		r.mu.Lock()
		r.svcActive = true
		r.svcReplaced = false
		r.mu.Unlock()
		return nil
	}
	r.mu.Lock()
	r.svcActive = false
	r.svcReplaced = false
	r.mu.Unlock()
	return r.process.Start(ctx, config)
}

func (r *Router) Stop(ctx context.Context) error {
	r.mu.Lock()
	r.svcActive = false
	r.svcReplaced = false
	r.mu.Unlock()
	if svc := r.serviceClient(); svc != nil {
		return svc.Stop(ctx)
	}
	return r.process.Stop(ctx)
}

func (r *Router) SetMode(ctx context.Context, mode string) error {
	if svc := r.serviceClient(); svc != nil {
		return svc.SetMode(ctx, mode)
	}
	return r.process.SetMode(ctx, mode)
}

func (r *Router) SelectNode(ctx context.Context, group, node string) error {
	if svc := r.serviceClient(); svc != nil {
		return svc.SelectNode(ctx, group, node)
	}
	return r.process.SelectNode(ctx, group, node)
}

func (r *Router) LatencyTest(ctx context.Context, ids []string) (map[string]int, error) {
	if svc := r.serviceClient(); svc != nil {
		return svc.LatencyTest(ctx, ids)
	}
	return r.process.LatencyTest(ctx, ids)
}

func (r *Router) Logs(ctx context.Context, after int64, limit int) ([]telemetry.LogEntry, error) {
	if svc := r.serviceClient(); svc != nil {
		r.mu.Lock()
		base := r.svcLogBase
		r.mu.Unlock()
		entries, err := svc.Logs(ctx, max(0, after-base), limit)
		if err != nil {
			return nil, err
		}
		return r.shiftAndTrackLogs(entries, base), nil
	}
	r.mu.Lock()
	base := r.procLogBase
	r.mu.Unlock()
	entries, err := r.process.Logs(ctx, max(0, after-base), limit)
	if err != nil {
		return nil, err
	}
	return r.shiftAndTrackLogs(entries, base), nil
}

func (r *Router) shiftAndTrackLogs(entries []telemetry.LogEntry, base int64) []telemetry.LogEntry {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := range entries {
		entries[i].Seq += base
		if entries[i].Seq > r.logHigh {
			r.logHigh = entries[i].Seq
		}
	}
	return entries
}
