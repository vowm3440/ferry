package app

import (
	"context"
	"errors"
	"time"
)

type recoveryState struct {
	desired     bool
	attempts    []time.Time
	proxyWanted bool
}
type processExit interface{ Exited() bool }

// RecoverCore reacts only to confirmed child process exit, not slow IPC.
// Caller schedules it; an intentional Stop always clears desired state.
func (s *Session) RecoverCore(ctx context.Context, now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	process, ok := s.remote.(processExit)
	if !ok || !s.recovery.desired || !process.Exited() {
		return nil
	}
	kept := s.recovery.attempts[:0]
	for _, at := range s.recovery.attempts {
		if now.Sub(at) < 5*time.Minute {
			kept = append(kept, at)
		}
	}
	s.recovery.attempts = kept
	if s.proxy != nil {
		on, _, err := s.proxy.Status()
		if err != nil {
			return err
		}
		s.recovery.proxyWanted = s.recovery.proxyWanted || on
		if err := s.proxy.Disable(); err != nil {
			return err
		}
	}
	if len(s.recovery.attempts) >= 3 {
		return errors.New("内核连续退出，已暂停自动重启，请手动重试")
	}
	s.recovery.attempts = append(s.recovery.attempts, now)
	if err := s.startConfig(ctx, s.config); err != nil {
		return errors.New("内核退出后自动重启失败，已恢复系统代理")
	}
	if err := s.restoreSelections(ctx, s.selected); err != nil {
		return err
	}
	if s.recovery.proxyWanted && s.proxy != nil {
		if err := s.proxy.Enable(int(s.config.Inbound.MixedPort)); err != nil {
			return err
		}
	}
	s.recovery.proxyWanted = false
	return nil
}
