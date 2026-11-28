package app

import (
	"context"
	"errors"
	"slices"
	"strings"

	"ferry/internal/ipc"
	"ferry/internal/routing"
	"ferry/internal/spec"
	"ferry/internal/telemetry"
)

type RuleSets struct{ session *Session }

func (s *Session) RuleSets() *RuleSets { return &RuleSets{s} }

func (r *RuleSets) Presets(context.Context) []spec.RuleSet {
	return routing.CommunityRuleSets()
}

func (r *RuleSets) syncCustomToCore(ctx context.Context, sets []spec.RuleSet) error {
	custom, ok := r.session.remote.(ipc.CustomRuleSetManager)
	if !ok {
		return nil
	}
	if sets == nil {
		sets = []spec.RuleSet{}
	}
	return custom.RuleSetSync(ctx, sets)
}

func (r *RuleSets) List(ctx context.Context, refresh bool) ([]telemetry.RuleSetInfo, error) {
	manager, ok := r.session.remote.(ipc.RuleSetManager)
	if !ok {
		return nil, errors.New("当前内核不支持规则集")
	}
	r.session.mu.Lock()
	sets := slices.Clone(r.session.config.RuleSets)
	r.session.mu.Unlock()
	if err := r.syncCustomToCore(ctx, sets); err != nil {
		return nil, err
	}
	list, err := manager.RuleSetList(ctx)
	if err != nil {
		return nil, err
	}
	return appendMissingCustomMeta(list, sets), nil
}

func appendMissingCustomMeta(list []telemetry.RuleSetInfo, sets []spec.RuleSet) []telemetry.RuleSetInfo {
	seen := make(map[string]bool, len(list))
	for _, item := range list {
		seen[item.ID] = true
	}
	for _, rs := range sets {
		if !seen[rs.ID] {
			list = append(list, telemetry.RuleSetInfo{
				ID: rs.ID, Name: rs.Name, URL: rs.URL, Status: "missing",
				Custom: true, AutoUpdate: rs.AutoUpdate, UpdateHours: rs.UpdateHours,
			})
		}
	}
	return list
}

func (r *RuleSets) Update(ctx context.Context, ids []string) error {
	maxBatch := len(spec.RuleSetCatalogIDs()) + spec.MaxCustomRuleSets
	if len(ids) == 0 || len(ids) > maxBatch {
		return errors.New("请选择有效的规则集数量")
	}
	manager, ok := r.session.remote.(ipc.RuleSetManager)
	if !ok {
		return errors.New("当前内核不支持规则集")
	}
	r.session.mu.Lock()
	config := r.session.snapshot().Config
	r.session.mu.Unlock()
	if err := validateRuleSetBatch(ids, config); err != nil {
		return err
	}
	if err := r.syncCustomToCore(ctx, config.RuleSets); err != nil {
		return err
	}
	for _, id := range ids {
		if err := manager.RuleSetUpdate(ctx, id); err != nil {
			return err
		}
	}
	return nil
}

func validateRuleSetBatch(ids []string, config spec.RunSpec) error {
	seen := map[string]bool{}
	for _, id := range ids {
		if !config.HasRuleSet(id) {
			return errors.New("规则集不在支持的目录内")
		}
		if seen[id] {
			return errors.New("规则集列表含重复项")
		}
		seen[id] = true
	}
	return nil
}

func (r *RuleSets) Save(ctx context.Context, rs spec.RuleSet) (spec.RuleSet, error) {
	rs.ID = strings.TrimSpace(rs.ID)
	rs.Name = strings.TrimSpace(rs.Name)
	rs.URL = strings.TrimSpace(rs.URL)
	if err := spec.ValidateRuleSet(rs); err != nil {
		return spec.RuleSet{}, err
	}
	r.session.mu.Lock()
	next := r.session.snapshot()
	idx := slices.IndexFunc(next.Config.RuleSets, func(item spec.RuleSet) bool { return item.ID == rs.ID })
	if idx >= 0 {
		next.Config.RuleSets[idx] = rs
	} else {
		next.Config.RuleSets = append(next.Config.RuleSets, rs)
	}
	if err := next.Config.Validate(); err != nil {
		r.session.mu.Unlock()
		return spec.RuleSet{}, err
	}
	err := r.session.commit(next)
	sets := slices.Clone(next.Config.RuleSets)
	version := r.session.revision
	r.session.mu.Unlock()
	if err != nil {
		return spec.RuleSet{}, err
	}
	_ = r.syncCustomToCore(ctx, sets)
	r.session.emit("rules:applied", map[string]uint64{"version": version})
	return rs, nil
}

func (r *RuleSets) Remove(ctx context.Context, id string) error {
	if spec.ValidRuleSetID(id) {
		return errors.New("内置规则集不可删除")
	}
	r.session.mu.Lock()
	next := r.session.snapshot()
	idx := slices.IndexFunc(next.Config.RuleSets, func(item spec.RuleSet) bool { return item.ID == id })
	if idx < 0 {
		r.session.mu.Unlock()
		return errors.New("自定义规则集不存在")
	}
	if ruleSetReferenced(next.Config, id) {
		r.session.mu.Unlock()
		return errors.New("仍有路由规则或分流 DNS 引用此规则集，请先移除引用")
	}
	next.Config.RuleSets = slices.Delete(next.Config.RuleSets, idx, idx+1)
	err := r.session.commit(next)
	sets := slices.Clone(next.Config.RuleSets)
	version := r.session.revision
	r.session.mu.Unlock()
	if err != nil {
		return err
	}
	_ = r.syncCustomToCore(ctx, sets)
	if custom, ok := r.session.remote.(ipc.CustomRuleSetManager); ok {
		_ = custom.RuleSetDelete(ctx, id)
	}
	r.session.emit("rules:applied", map[string]uint64{"version": version})
	return nil
}

func ruleSetReferenced(config spec.RunSpec, id string) bool {
	ref := spec.CustomRuleSetPrefix + id
	for _, rule := range config.Rules {
		if slices.Contains(rule.Hosts, ref) {
			return true
		}
	}
	for _, dnsRule := range config.DNS.Rules {
		if slices.Contains(dnsRule.Domains, ref) {
			return true
		}
	}
	return false
}
