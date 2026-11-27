package app

import (
	"context"
	"errors"
	"sync"

	"ferry/internal/ipc"
	"ferry/internal/telemetry"
)

type latencyValue struct {
	Milliseconds int    `json:"latencyMs"`
	Status       string `json:"status"`
}
type inspectionState struct {
	mu                sync.Mutex
	latencies         map[string]latencyValue
	connectionsPaused bool
	logsPaused        bool
	previous          map[string]telemetry.Connection
	logs              []telemetry.LogEntry
	logCursor         int64
}

func (s *Session) inspector() (ipc.Inspector, error) {
	inspector, ok := s.remote.(ipc.Inspector)
	if !ok {
		return nil, errors.New("当前内核不支持此功能")
	}
	return inspector, nil
}

func (c *Core) LatencyTest(ctx context.Context, ids []string) (map[string]int, error) {
	c.session.mu.Lock()
	if len(ids) == 0 || len(ids) > 1024 {
		c.session.mu.Unlock()
		return nil, errors.New("请选择需要测速的节点")
	}
	known := map[string]bool{}
	for _, node := range c.session.config.Nodes {
		known[node.ID] = true
	}
	for _, id := range ids {
		if !known[id] {
			c.session.mu.Unlock()
			return nil, errors.New("测速节点不存在")
		}
	}
	inspector, err := c.session.inspector()
	c.session.mu.Unlock()
	if err != nil {
		return nil, err
	}
	result := map[string]int{}
	for start := 0; start < len(ids); start += 32 {
		batch := ids[start:min(start+32, len(ids))]
		c.session.markTesting(batch)
		values, err := inspector.LatencyTest(ctx, batch)
		for _, id := range batch {
			result[id] = -1
			if value, ok := values[id]; ok {
				result[id] = value
			}
		}
		c.session.markLatency(batch, result)
		if err != nil {
			return result, err
		}
	}
	return result, nil
}

func (s *Session) markTesting(ids []string) {
	values := map[string]latencyValue{}
	for _, id := range ids {
		values[id] = latencyValue{Milliseconds: -1, Status: "testing"}
	}
	s.setLatencies(values)
}
func (s *Session) markLatency(ids []string, delays map[string]int) {
	values := map[string]latencyValue{}
	for _, id := range ids {
		status := "ok"
		if delays[id] < 0 {
			status = "timeout"
		}
		values[id] = latencyValue{Milliseconds: delays[id], Status: status}
	}
	s.setLatencies(values)
}
func (s *Session) setLatencies(values map[string]latencyValue) {
	s.inspection.mu.Lock()
	if s.inspection.latencies == nil {
		s.inspection.latencies = map[string]latencyValue{}
	}
	for id, value := range values {
		s.inspection.latencies[id] = value
	}
	s.inspection.mu.Unlock()
	s.emit("nodes:latency", values)
}
