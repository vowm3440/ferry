package app

import (
	"context"
	"errors"
	"ferry/internal/ipc"
	"ferry/internal/spec"
	"ferry/internal/telemetry"
)

type RuleSets struct{ session *Session }

func (s *Session) RuleSets() *RuleSets { return &RuleSets{s} }

func (r *RuleSets) List(ctx context.Context, refresh bool) ([]telemetry.RuleSetInfo, error) {
	manager, ok := r.session.remote.(ipc.RuleSetManager)
	if !ok {
		return nil, errors.New("当前内核不支持规则集")
	}
	return manager.RuleSetList(ctx) // refresh re-reads local metadata; never implicitly downloads.
}
func (r *RuleSets) Update(ctx context.Context, ids []string) error {
	if len(ids) == 0 || len(ids) > 4 {
		return errors.New("请选择 1 到 4 个规则集")
	}
	manager, ok := r.session.remote.(ipc.RuleSetManager)
	if !ok {
		return errors.New("当前内核不支持规则集")
	}
	seen := map[string]bool{}
	for _, id := range ids {
		if !spec.ValidRuleSetID(id) {
			return errors.New("规则集不在支持的目录内")
		}
		if seen[id] {
			return errors.New("规则集列表含重复项")
		}
		seen[id] = true
	}
	for _, id := range ids {
		if err := manager.RuleSetUpdate(ctx, id); err != nil {
			return err
		}
	}
	return nil
}
