package host

import (
	"context"
	"errors"
	"ferry/internal/core/rulesets"
	"ferry/internal/release"
	"ferry/internal/spec"
	"sync"
	"time"
)

type Host struct {
	mu           sync.RWMutex
	operation    chan struct{}
	current      *instance
	state        State
	traffic      Traffic
	sampledAt     time.Time
	rateUpload    int64
	rateDownload  int64
	totalUpload   int64
	totalDownload int64
	logs         *logBuffer
	ruleSets     *rulesets.Manager
	serviceMode  bool
	hasIPv6      func() bool
}

var errCanceled = errors.New("操作已取消或超时")
var errStopped = errors.New("内核尚未运行")

func New() *Host {
	return &Host{
		operation: make(chan struct{}, 1),
		state:     State{Status: "stopped", Mode: "rule", Version: release.Version, CoreVersion: "1.14.4"},
		traffic:   zeroTraffic(),
		logs:      newLogBuffer(),
	}
}

func NewService(ruleSetsDir string) *Host {
	h := NewWithRuleSets(ruleSetsDir)
	h.serviceMode = true
	h.state.ServiceMode = true
	return h
}

// A timed-out caller can leave while cleanup continues under the operation lock.
// Subsequent operations cannot overtake that cleanup or touch a half-closed box.
func (h *Host) operate(ctx context.Context, action func() error) error {
	if ctx.Err() != nil {
		return errCanceled
	}
	select {
	case h.operation <- struct{}{}:
	case <-ctx.Done():
		return errCanceled
	}
	result := make(chan error, 1)
	go func() { defer func() { <-h.operation }(); result <- action() }()
	select {
	case err := <-result:
		return err
	case <-ctx.Done():
		return errCanceled
	}
}

func (h *Host) Status() State {
	h.mu.RLock()
	defer h.mu.RUnlock()
	state := h.state
	if h.current != nil {
		state.UptimeMs = time.Since(h.current.started).Milliseconds()
	}
	return state
}

func (h *Host) Start(ctx context.Context, s spec.RunSpec) error {
	return h.operate(ctx, func() error { return h.start(ctx, s) })
}

func (h *Host) Stop(ctx context.Context) error {
	return h.operate(ctx, h.stop)
}
