package app

import (
	"context"
	"errors"
)

// Used for mutations; display state() intentionally converts transport errors
// to an error screen and must not be used as evidence that the core is stopped.
func (s *Session) knownStatus(ctx context.Context) (string, error) {
	if ctx.Err() != nil {
		return "", errors.New("操作已取消")
	}
	state, err := s.remote.Status(ctx)
	if err != nil {
		return "", errors.New("无法确认内核状态，配置未更改")
	}
	if state.Status != "stopped" && state.Status != "running" && state.Status != "error" && state.Status != "starting" {
		return "", errors.New("内核状态无效")
	}
	return state.Status, nil
}

func (s *Session) requireStopped(ctx context.Context, message string) error {
	status, err := s.knownStatus(ctx)
	if err != nil {
		return err
	}
	if status == "running" || status == "starting" {
		return errors.New(message)
	}
	return nil
}
