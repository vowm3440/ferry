package main

import (
	"context"
	"ferry/internal/telemetry"
)

func (a *adapter) LatencyTest(ctx context.Context, ids []string) (map[string]int, error) {
	return a.core.LatencyTest(ctx, ids)
}
func (a *adapter) Connections(ctx context.Context) ([]telemetry.Connection, error) {
	return a.core.Connections(ctx)
}
func (a *adapter) CloseConnection(ctx context.Context, id string) error {
	return a.core.CloseConnection(ctx, id)
}
func (a *adapter) Logs(ctx context.Context, after int64, limit int) ([]telemetry.LogEntry, error) {
	return a.core.Logs(ctx, after, limit)
}
func (a *adapter) SetLogLevel(ctx context.Context, level string) error {
	return a.core.SetLogLevel(ctx, level)
}

func (a *adapter) TestRule(ctx context.Context, input telemetry.RuleTestRequest) (telemetry.RuleTestResult, error) {
	return a.core.TestRule(ctx, input)
}

func (a *adapter) RuleSetList(ctx context.Context) ([]telemetry.RuleSetInfo, error) {
	return a.core.RuleSetList(ctx)
}
func (a *adapter) RuleSetUpdate(ctx context.Context, id string) error {
	return a.core.RuleSetUpdate(ctx, id)
}
