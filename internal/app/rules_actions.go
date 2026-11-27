package app

import (
	"context"
	"errors"
	"strconv"

	"ferry/internal/routing"
	"ferry/internal/spec"
	"ferry/internal/telemetry"
)

// Mutations are transactional and immediately apply. Apply validates the full
// current ID order and explicitly reapplies it; it never applies a partial set.
func (r *Rules) Apply(ctx context.Context, ids []string) error {
	return r.mutate(ctx, func(current []spec.Rule) ([]spec.Rule, error) { return reorderRules(current, ids) })
}

func (r *Rules) CreateFromConnection(ctx context.Context, id string) (spec.Rule, error) {
	items, err := r.session.Connections().Snapshot(ctx)
	if err != nil {
		return spec.Rule{}, err
	}
	for _, item := range items {
		if item.ID != id {
			continue
		}
		rule := spec.Rule{Name: "连接规则", Enabled: true, Apps: []string{}, Hosts: []string{}, Ports: []string{}, Network: "any", Action: "proxy", TargetID: "proxy", ResolveBefore: true}
		if item.ProcessPath != "" {
			rule.Apps = []string{item.ProcessPath}
		}
		if item.Host != "" {
			rule.Hosts = []string{item.Host}
		}
		if item.Port > 0 {
			rule.Ports = []string{strconv.Itoa(item.Port)}
		}
		if item.Action == "direct" || item.Action == "block" {
			rule.Action = item.Action
			rule.TargetID = ""
		}
		if rule.Action == "proxy" {
			r.session.mu.Lock()
			if len(r.session.config.Groups) > 0 {
				rule.TargetID = r.session.config.Groups[0].ID
			}
			r.session.mu.Unlock()
		}
		return rule, nil
	}
	return spec.Rule{}, errors.New("连接已结束或不存在")
}

type ruleTester interface {
	TestRule(context.Context, telemetry.RuleTestRequest) (telemetry.RuleTestResult, error)
}

func (r *Rules) Import(_ context.Context, kind, content string) (routing.ImportPreview, error) {
	r.session.mu.Lock()
	config := r.session.snapshot().Config
	r.session.mu.Unlock()
	return routing.Import(kind, content, config)
}

func (r *Rules) Test(ctx context.Context, application, host string, port int, network string) (telemetry.RuleTestResult, error) {
	tester, ok := r.session.remote.(ruleTester)
	if !ok {
		return telemetry.RuleTestResult{}, errors.New("当前内核不支持规则测试")
	}
	r.session.mu.Lock()
	config := r.session.snapshot().Config
	config.Rules = normalizeRules(config.Rules)
	r.session.mu.Unlock()
	return tester.TestRule(ctx, telemetry.RuleTestRequest{Config: config, App: application, Host: host, Port: port, Network: network})
}
