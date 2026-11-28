package app

import (
	"context"
	"errors"
	"slices"

	"ferry/internal/ipc"
	"ferry/internal/routing"
	"ferry/internal/spec"
)

type ApplyPresetResult struct {
	RuleCount       int      `json:"ruleCount"`
	RuleSetCount    int      `json:"ruleSetCount"`
	AppGroupCount   int      `json:"appGroupCount"`
	DNSRuleCount    int      `json:"dnsRuleCount"`
	MissingRuleSets []string `json:"missingRuleSets"`
}

func (r *Rules) DefaultLazyPreset(context.Context) routing.LazyPresetOptions {
	return routing.DefaultLazyPresetOptions()
}

// ApplyLazyPreset writes the curated split-routing rules, community SRS sets,
// app groups, and split DNS atomically. When the core is running, missing
// referenced rule-sets are downloaded first so core restart succeeds cleanly.
func (r *Rules) ApplyLazyPreset(ctx context.Context, opts routing.LazyPresetOptions) (ApplyPresetResult, error) {
	r.session.mu.Lock()
	next := r.session.snapshot()
	r.session.mu.Unlock()
	built, err := routing.BuildLazyPreset(next.Config, opts)
	if err != nil {
		return ApplyPresetResult{}, err
	}
	next.Config = built
	next.Settings = fromSpec(next.Settings, built)
	next.Settings.RuleSetAutoUpdate = true
	if next.Settings.RuleSetUpdateHours <= 0 {
		next.Settings.RuleSetUpdateHours = 24
	}
	missing, err := r.preparePresetRuleSets(ctx, built)
	if err != nil {
		return ApplyPresetResult{}, err
	}
	if err := r.commitPreset(ctx, next); err != nil {
		return ApplyPresetResult{}, err
	}
	return ApplyPresetResult{
		RuleCount:       len(built.Rules),
		RuleSetCount:    len(built.RuleSets),
		AppGroupCount:   len(built.AppGroups),
		DNSRuleCount:    len(built.DNS.Rules),
		MissingRuleSets: missing,
	}, nil
}

func (r *Rules) preparePresetRuleSets(ctx context.Context, config spec.RunSpec) ([]string, error) {
	manager, ok := r.session.remote.(ipc.RuleSetManager)
	if !ok {
		return []string{}, nil
	}
	_ = r.session.RuleSets().syncCustomToCore(ctx, config.RuleSets)
	list, err := manager.RuleSetList(ctx)
	if err != nil {
		return []string{}, nil
	}
	ready := map[string]bool{}
	for _, item := range list {
		if item.Status == "ready" {
			ready[item.ID] = true
		}
	}
	var missing []string
	for _, id := range config.ReferencedRuleSets() {
		if !ready[id] {
			missing = append(missing, id)
		}
	}
	state, stateErr := r.session.remote.Status(ctx)
	if stateErr == nil && state.Status == "running" && len(missing) > 0 {
		for _, id := range missing {
			if err := manager.RuleSetUpdate(ctx, id); err != nil {
				return nil, errors.New("运行中应用预设需先下载缺失规则集，下载失败：" + id + "（可先停止内核后再应用并手动下载）")
			}
		}
		return []string{}, nil
	}
	if missing == nil {
		missing = []string{}
	}
	return missing, nil
}

func (r *Rules) commitPreset(ctx context.Context, next savedSession) error {
	r.session.mu.Lock()
	err := validateImportedConfig(next.Config)
	if err == nil {
		err = r.session.reconfigure(ctx, next)
	}
	version := r.session.revision
	sets := slices.Clone(next.Config.RuleSets)
	r.session.mu.Unlock()
	if err != nil {
		return err
	}
	_ = r.session.RuleSets().syncCustomToCore(ctx, sets)
	r.session.emit("rules:applied", map[string]uint64{"version": version})
	return nil
}
