package app

import (
	"context"
	"errors"
	"reflect"
	"sort"

	"ferry/internal/telemetry"
)

type Connections struct{ session *Session }

func (s *Session) Connections() *Connections { return &Connections{s} }
func (c *Connections) Pause(_ context.Context, on bool) error {
	c.session.inspection.mu.Lock()
	defer c.session.inspection.mu.Unlock()
	c.session.inspection.connectionsPaused = on
	return nil
}
func (c *Connections) Snapshot(ctx context.Context) ([]telemetry.Connection, error) {
	inspector, err := c.session.inspector()
	if err != nil {
		return nil, err
	}
	return inspector.Connections(ctx)
}
func (c *Connections) Close(ctx context.Context, id string) error {
	if id == "" {
		return errors.New("请选择连接")
	}
	inspector, err := c.session.inspector()
	if err != nil {
		return err
	}
	return inspector.CloseConnection(ctx, id)
}
func (c *Connections) CloseAll(ctx context.Context) error {
	inspector, err := c.session.inspector()
	if err != nil {
		return err
	}
	return inspector.CloseConnection(ctx, "")
}

type connectionDelta struct {
	Items   []telemetry.Connection `json:"items"`
	Added   []telemetry.Connection `json:"added"`
	Updated []telemetry.Connection `json:"updated"`
	Removed []string               `json:"removed"`
}

func (s *Session) pollConnections(ctx context.Context) {
	s.inspection.mu.Lock()
	paused := s.inspection.connectionsPaused
	s.inspection.mu.Unlock()
	if paused {
		return
	}
	items, err := s.Connections().Snapshot(ctx)
	if err != nil {
		return
	}
	delta := connectionDelta{Items: items, Added: []telemetry.Connection{}, Updated: []telemetry.Connection{}, Removed: []string{}}
	next := map[string]telemetry.Connection{}
	s.inspection.mu.Lock()
	for _, item := range items {
		next[item.ID] = item
		old, ok := s.inspection.previous[item.ID]
		if !ok {
			delta.Added = append(delta.Added, item)
		} else if !reflect.DeepEqual(old, item) {
			delta.Updated = append(delta.Updated, item)
		}
	}
	for id := range s.inspection.previous {
		if _, ok := next[id]; !ok {
			delta.Removed = append(delta.Removed, id)
		}
	}
	s.inspection.previous = next
	s.inspection.mu.Unlock()
	sort.Strings(delta.Removed)
	s.emit("connections:snapshot", delta)
}

func (s *Session) PollTelemetry(ctx context.Context) {
	s.pollConnections(ctx)
	_ = s.pollLogs(ctx, true) // Remote errors are reflected by core:state; never invent log entries.
}
