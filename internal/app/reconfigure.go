package app

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"ferry/internal/spec"
)

// Called with session.mu held. A failed live refresh restarts the previous
// configuration; the persisted file changes only after the new core started.
func (s *Session) reconfigure(ctx context.Context, next savedSession) error {
	if ctx.Err() != nil {
		return errors.New("配置更改已取消")
	}
	state, err := s.remote.Status(ctx)
	if err != nil {
		return errors.New("无法确认内核状态，配置未更改")
	}
	if state.Status == "starting" {
		return errors.New("内核正在启动，请稍后重试")
	}
	if state.Status != "running" {
		return s.commit(next)
	}
	if err := s.remote.Stop(ctx); err != nil {
		return err
	}
	if err := s.startConfig(ctx, next.Config); err != nil {
		return s.restoreCore(err)
	}
	if err := s.restoreSelections(ctx, next.Selected); err != nil {
		return s.restoreCore(err)
	}
	if err := s.commit(next); err != nil {
		return s.restoreCore(err)
	}
	return nil
}

func (s *Session) startConfig(ctx context.Context, config spec.RunSpec) error {
	data, err := json.Marshal(config)
	if err != nil {
		return errors.New("无法生成内核配置")
	}
	return s.remote.Start(ctx, data)
}

func (s *Session) restoreCore(cause error) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := s.remote.Stop(ctx); err != nil {
		return errors.New("新配置应用失败且内核未能停止，请重启应用")
	}
	if err := s.startConfig(ctx, s.config); err != nil {
		return errors.New("新配置应用失败，原配置已保留但内核恢复失败，请重新启动内核")
	}
	if err := s.restoreSelections(ctx, s.selected); err != nil {
		return errors.New("原配置已恢复，但节点选择恢复失败，请重新选择节点")
	}
	return cause
}

func (s *Session) restoreSelections(ctx context.Context, selected map[string]string) error {
	for group, node := range selected {
		if err := s.remote.SelectNode(ctx, group, node); err != nil {
			return errors.New("无法恢复节点选择")
		}
	}
	return nil
}
