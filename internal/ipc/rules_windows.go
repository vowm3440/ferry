//go:build windows

package ipc

import (
	"context"
	"ferry/internal/telemetry"
)

func (p *Process) TestRule(ctx context.Context, input telemetry.RuleTestRequest) (telemetry.RuleTestResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := p.connect(ctx); err != nil {
		return telemetry.RuleTestResult{}, err
	}
	return p.client.TestRule(ctx, input)
}
