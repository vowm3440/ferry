package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"slices"

	"ferry/internal/spec"
)

type Rules struct{ session *Session }

func (s *Session) Rules() *Rules { return &Rules{s} }

func (r *Rules) List(context.Context) ([]spec.Rule, error) {
	r.session.mu.Lock()
	defer r.session.mu.Unlock()
	return normalizeRules(r.session.config.Rules), nil
}

func normalizeRules(rules []spec.Rule) []spec.Rule {
	defaults := spec.DefaultRules()
	result := []spec.Rule{defaults[0]}
	for _, rule := range rules {
		if rule.ID == defaults[0].ID || rule.ID == defaults[1].ID {
			continue
		}
		rule.Apps = slices.Clone(rule.Apps)
		rule.Hosts = slices.Clone(rule.Hosts)
		rule.Ports = slices.Clone(rule.Ports)
		if rule.Apps == nil {
			rule.Apps = []string{}
		}
		if rule.Hosts == nil {
			rule.Hosts = []string{}
		}
		if rule.Ports == nil {
			rule.Ports = []string{}
		}
		result = append(result, rule)
	}
	return append(result, defaults[1])
}

func (r *Rules) Save(ctx context.Context, rule spec.Rule) (spec.Rule, error) {
	if rule.Apps == nil {
		rule.Apps = []string{}
	}
	if rule.Hosts == nil {
		rule.Hosts = []string{}
	}
	if rule.Ports == nil {
		rule.Ports = []string{}
	}
	if rule.Pinned || rule.ID == "builtin-lan" || rule.ID == "builtin-default" {
		return spec.Rule{}, errors.New("固定规则不可编辑")
	}
	if rule.ID == "" {
		var id [12]byte
		if _, err := rand.Read(id[:]); err != nil {
			return spec.Rule{}, errors.New("无法创建规则标识")
		}
		rule.ID = "r-" + hex.EncodeToString(id[:])
	}
	err := r.mutate(ctx, func(current []spec.Rule) ([]spec.Rule, error) {
		for i, v := range current {
			if v.ID == rule.ID {
				current[i] = rule
				return current, nil
			}
		}
		return slices.Insert(current, len(current)-1, rule), nil
	})
	if err != nil {
		return spec.Rule{}, err
	}
	return rule, nil
}

func (r *Rules) Remove(ctx context.Context, id string) error {
	return r.mutate(ctx, func(current []spec.Rule) ([]spec.Rule, error) {
		for i, v := range current {
			if v.ID == id {
				if v.Pinned {
					return nil, errors.New("固定规则不可删除")
				}
				return slices.Delete(current, i, i+1), nil
			}
		}
		return nil, errors.New("规则不存在")
	})
}

func (r *Rules) SetEnabled(ctx context.Context, id string, on bool) error {
	return r.mutate(ctx, func(current []spec.Rule) ([]spec.Rule, error) {
		for i, v := range current {
			if v.ID == id {
				if v.Pinned {
					return nil, errors.New("固定规则不可禁用")
				}
				current[i].Enabled = on
				return current, nil
			}
		}
		return nil, errors.New("规则不存在")
	})
}

func (r *Rules) Reorder(ctx context.Context, ids []string) error {
	return r.mutate(ctx, func(current []spec.Rule) ([]spec.Rule, error) { return reorderRules(current, ids) })
}

func reorderRules(current []spec.Rule, ids []string) ([]spec.Rule, error) {
	if len(ids) != len(current) || len(ids) < 2 || ids[0] != "builtin-lan" || ids[len(ids)-1] != "builtin-default" {
		return nil, errors.New("规则顺序无效，固定规则须位于首尾")
	}
	byID := map[string]spec.Rule{}
	for _, v := range current {
		byID[v.ID] = v
	}
	result := make([]spec.Rule, 0, len(ids))
	for _, id := range ids {
		v, ok := byID[id]
		if !ok {
			return nil, errors.New("规则顺序含重复或未知标识")
		}
		delete(byID, id)
		result = append(result, v)
	}
	return result, nil
}

func (r *Rules) mutate(ctx context.Context, change func([]spec.Rule) ([]spec.Rule, error)) error {
	r.session.mu.Lock()
	next := r.session.snapshot()
	rules, err := change(normalizeRules(next.Config.Rules))
	if err == nil {
		next.Config.Rules = rules
		err = next.Config.Validate()
	}
	if err == nil {
		err = validateImportedConfig(next.Config)
	}
	if err == nil {
		err = r.session.reconfigure(ctx, next)
	}
	version := r.session.revision
	r.session.mu.Unlock()
	if err == nil {
		r.session.emit("rules:applied", map[string]uint64{"version": version})
	}
	return err
}
