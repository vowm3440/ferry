package app

import (
	"context"
	"ferry/internal/telemetry"
	"testing"
)

type inspectedRemote struct {
	remoteFixture
	connections []telemetry.Connection
	logs        []telemetry.LogEntry
}

func (r *inspectedRemote) LatencyTest(context.Context, []string) (map[string]int, error) {
	return map[string]int{"test": 42}, nil
}
func (r *inspectedRemote) Connections(context.Context) ([]telemetry.Connection, error) {
	return r.connections, nil
}
func (r *inspectedRemote) CloseConnection(_ context.Context, id string) error {
	r.connections = nil
	return nil
}
func (r *inspectedRemote) Logs(_ context.Context, after int64, limit int) ([]telemetry.LogEntry, error) {
	result := []telemetry.LogEntry{}
	for _, entry := range r.logs {
		if entry.Seq > after && len(result) < limit {
			result = append(result, entry)
		}
	}
	return result, nil
}
func (r *inspectedRemote) SetLogLevel(context.Context, string) error { return nil }

func TestInspectionServicesExposeMeasurementsAndRetainLogs(t *testing.T) {
	session, fixture := configured(t)
	remote := &inspectedRemote{remoteFixture: *fixture, connections: []telemetry.Connection{{ID: "connection", Host: "example.com", Port: 443, Chain: []string{"test"}}}, logs: []telemetry.LogEntry{{Seq: 1, Level: "info", Message: "test entry"}}}
	session.remote = remote
	core, nodes, _ := session.Services()
	ctx := context.Background()
	if err := core.Start(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := core.LatencyTest(ctx, []string{"test"}); err != nil {
		t.Fatal(err)
	}
	list, _ := nodes.List(ctx)
	if list.Nodes[0].LatencyMs != 42 || list.Nodes[0].LatencyStatus != "ok" {
		t.Fatal("measured latency not exposed")
	}
	connections, err := session.Connections().Snapshot(ctx)
	if err != nil || len(connections) != 1 {
		t.Fatal("connection snapshot missing")
	}
	if err := session.Connections().Close(ctx, "connection"); err != nil {
		t.Fatal(err)
	}
	connections, _ = session.Connections().Snapshot(ctx)
	if len(connections) != 0 {
		t.Fatal("connection close did not reach core")
	}
	entries, err := session.Logs().Snapshot(ctx, 50)
	if err != nil || len(entries) != 1 || entries[0].Message != "test entry" {
		t.Fatal("log snapshot missing")
	}
	entries, err = session.Logs().Snapshot(ctx, 50)
	if err != nil || len(entries) != 1 {
		t.Fatal("log polling duplicated entries")
	}
}
