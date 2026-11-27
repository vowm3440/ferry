package app

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"ferry/internal/telemetry"
)

func TestTelemetryDeltasPauseAndLogExport(t *testing.T) {
	s, fixture := configured(t)
	remote := &inspectedRemote{remoteFixture: *fixture}
	s.remote = remote
	ctx := context.Background()
	var deltas []connectionDelta
	var batches [][]telemetry.LogEntry
	s.SetEmitter(func(name string, value any) {
		if name == "connections:snapshot" {
			deltas = append(deltas, value.(connectionDelta))
		}
		if name == "logs:entry" {
			batches = append(batches, value.([]telemetry.LogEntry))
		}
	})
	remote.connections = []telemetry.Connection{{ID: "a", Host: "one.example", Port: 443, Chain: []string{}}}
	s.PollTelemetry(ctx)
	if len(deltas) != 1 || len(deltas[0].Added) != 1 {
		t.Fatal("missing added connection")
	}
	remote.connections[0].Up = 10
	s.PollTelemetry(ctx)
	if len(deltas[1].Updated) != 1 || deltas[1].Updated[0].Up != 10 {
		t.Fatal("missing updated connection")
	}
	if err := s.Connections().Pause(ctx, true); err != nil {
		t.Fatal(err)
	}
	remote.connections = nil
	s.PollTelemetry(ctx)
	if len(deltas) != 2 {
		t.Fatal("paused connection push continued")
	}
	if err := s.Connections().Pause(ctx, false); err != nil {
		t.Fatal(err)
	}
	s.PollTelemetry(ctx)
	if len(deltas[2].Removed) != 1 || deltas[2].Removed[0] != "a" {
		t.Fatal("missing removed connection")
	}
	for i := 1; i <= 45; i++ {
		remote.logs = append(remote.logs, telemetry.LogEntry{Seq: int64(i), Level: "info", Message: "line"})
	}
	s.PollTelemetry(ctx)
	if len(batches) != 3 || len(batches[0]) != 20 || len(batches[2]) != 5 {
		t.Fatal("logs not batched at 20")
	}
	path := filepath.Join(t.TempDir(), "logs.jsonl")
	if err := s.Logs().Export(ctx, path); err != nil {
		t.Fatal(err)
	}
	if err := s.Logs().Export(ctx, path); err == nil {
		t.Fatal("export overwrote existing file")
	}
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 {
		t.Fatal("empty export")
	}
	lines := bytes.Split(bytes.TrimSpace(data), []byte{'\n'})
	if len(lines) != 45 {
		t.Fatal("export omitted retained log entries")
	}
	var item telemetry.LogEntry
	if json.Unmarshal(lines[44], &item) != nil || item.Seq != 45 {
		t.Fatal("export contains invalid or reordered entries")
	}
	if err := s.Logs().Pause(ctx, true); err != nil {
		t.Fatal(err)
	}
	remote.logs = append(remote.logs, telemetry.LogEntry{Seq: 46, Level: "info", Message: "paused"})
	s.PollTelemetry(ctx)
	if len(batches) != 3 {
		t.Fatal("paused log push continued")
	}
	all, err := s.Logs().Snapshot(ctx, 5000)
	if err != nil || len(all) != 46 {
		t.Fatal("paused logs should remain queryable")
	}
}
