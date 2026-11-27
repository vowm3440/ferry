//go:build windows

package ipc

import (
	"context"
	"ferry/internal/telemetry"
)

func (p *Process) RuleSetList(ctx context.Context) ([]telemetry.RuleSetInfo, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := p.connect(ctx); err != nil {
		return nil, err
	}
	return p.client.RuleSetList(ctx)
}
func (p *Process) RuleSetUpdate(ctx context.Context, id string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := p.connect(ctx); err != nil {
		return err
	}
	return p.client.RuleSetUpdate(ctx, id)
}
