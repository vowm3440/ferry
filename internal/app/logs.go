package app

import (
	"context"
	"encoding/json"
	"errors"
	"ferry/internal/telemetry"
	"os"
	"slices"
	"time"
)

type Logs struct{ session *Session }

func (s *Session) Logs() *Logs { return &Logs{s} }
func (l *Logs) Pause(_ context.Context, on bool) error {
	l.session.inspection.mu.Lock()
	defer l.session.inspection.mu.Unlock()
	l.session.inspection.logsPaused = on
	return nil
}

func (l *Logs) Snapshot(ctx context.Context, limit int) ([]telemetry.LogEntry, error) {
	if limit < 1 || limit > 5000 {
		return nil, errors.New("日志数量须在 1 到 5000 之间")
	}
	if err := l.session.pollLogs(ctx, false); err != nil {
		return nil, err
	}
	l.session.inspection.mu.Lock()
	defer l.session.inspection.mu.Unlock()
	entries := l.session.inspection.logs
	result := append([]telemetry.LogEntry{}, entries[max(0, len(entries)-limit):]...)
	return result, nil
}

func (s *Session) pollLogs(ctx context.Context, push bool) error {
	inspector, err := s.inspector()
	if err != nil {
		return err
	}
	s.inspection.mu.Lock()
	after := s.inspection.logCursor
	s.inspection.mu.Unlock()
	entries, err := inspector.Logs(ctx, after, 200)
	if err != nil {
		return err
	}
	s.inspection.mu.Lock()
	fresh := []telemetry.LogEntry{}
	for _, entry := range entries {
		if entry.Seq > s.inspection.logCursor {
			s.inspection.logCursor = entry.Seq
			fresh = append(fresh, entry)
		}
	}
	s.inspection.logs = append(s.inspection.logs, fresh...)
	if len(s.inspection.logs) > 5000 {
		s.inspection.logs = slices.Clone(s.inspection.logs[len(s.inspection.logs)-5000:])
	}
	paused := s.inspection.logsPaused
	s.inspection.mu.Unlock()
	if push && !paused {
		for start := 0; start < len(fresh); start += 20 {
			s.emit("logs:entry", fresh[start:min(start+20, len(fresh))])
		}
	}
	return nil
}

func (l *Logs) SetLevel(ctx context.Context, level string) error {
	l.session.mu.Lock()
	defer l.session.mu.Unlock()
	next := l.session.snapshot()
	next.Config.Log.Level = level
	if err := next.Config.Validate(); err != nil {
		return errors.New("日志级别无效")
	}
	state, err := l.session.remote.Status(ctx)
	if err != nil {
		return errors.New("无法确认内核状态，日志设置未更改")
	}
	if state.Status == "starting" {
		return errors.New("内核正在启动，请稍后重试")
	}
	running := state.Status == "running"
	if running {
		inspector, err := l.session.inspector()
		if err != nil {
			return err
		}
		if err := inspector.SetLogLevel(ctx, level); err != nil {
			return err
		}
	}
	next.Settings.LogLevel = level
	if err := l.session.commit(next); err != nil {
		if running {
			inspector, inspectErr := l.session.inspector()
			rollback, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			if inspectErr != nil || inspector.SetLogLevel(rollback, l.session.config.Log.Level) != nil {
				return errors.New("日志设置保存失败且回退失败，请重新启动内核")
			}
		}
		return err
	}
	return nil
}

func (l *Logs) Export(ctx context.Context, path string) error {
	entries, err := l.Snapshot(ctx, 5000)
	if err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return errors.New("无法创建日志文件，请选择尚不存在的文件")
	}
	defer file.Close()
	encoder := json.NewEncoder(file)
	for _, entry := range entries {
		if err := encoder.Encode(entry); err != nil {
			return errors.New("日志导出失败")
		}
	}
	if err := file.Sync(); err != nil {
		return errors.New("日志文件写入失败")
	}
	return nil
}
