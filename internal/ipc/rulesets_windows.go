//go:build windows

package ipc

import (
	"context"
	"ferry/internal/spec"
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
func (p *Process) RuleSetSync(ctx context.Context, sets []spec.RuleSet) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := p.connect(ctx); err != nil {
		return err
	}
	return p.client.RuleSetSync(ctx, sets)
}
func (p *Process) RuleSetDelete(ctx context.Context, id string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := p.connect(ctx); err != nil {
		return err
	}
	return p.client.RuleSetDelete(ctx, id)
}
