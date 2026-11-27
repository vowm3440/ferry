package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"slices"

	"ferry/internal/spec"
)

func newRuleID() (string, error) {
	var id [12]byte
	if _, err := rand.Read(id[:]); err != nil {
		return "", errors.New("无法创建规则标识")
	}
	return "r-" + hex.EncodeToString(id[:]), nil
}

// SaveMany appends new rules (an import) as one transaction: the whole batch
// is validated and applied once, so a running core restarts a single time, or
// nothing is written at all. Caller-supplied IDs are preview placeholders and
// are always replaced.
func (r *Rules) SaveMany(ctx context.Context, rules []spec.Rule) error {
	if len(rules) == 0 {
		return errors.New("没有可导入的规则")
	}
	if len(rules) > spec.MaxRules {
		return errors.New("规则数量超过限制")
	}
	fresh := make([]spec.Rule, 0, len(rules))
	for _, rule := range rules {
		if rule.Pinned || rule.ID == "builtin-lan" || rule.ID == "builtin-default" {
			return errors.New("固定规则不可编辑")
		}
		id, err := newRuleID()
		if err != nil {
			return err
		}
		rule.ID = id
		rule.Apps = append([]string{}, rule.Apps...)
		rule.Hosts = append([]string{}, rule.Hosts...)
		rule.Ports = append([]string{}, rule.Ports...)
		fresh = append(fresh, rule)
	}
	return r.mutate(ctx, func(current []spec.Rule) ([]spec.Rule, error) {
		return slices.Insert(current, len(current)-1, fresh...), nil
	})
}
