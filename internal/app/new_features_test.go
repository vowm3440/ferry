package app

import (
	"context"
	"testing"
	"time"

	"ferry/internal/routing"
	"ferry/internal/spec"
	"ferry/internal/telemetry"
)

type customRuleSetRemote struct {
	catalogueRemote
	synced  []spec.RuleSet
	deleted []string
}

func (r *customRuleSetRemote) RuleSetSync(_ context.Context, sets []spec.RuleSet) error {
	r.synced = append([]spec.RuleSet(nil), sets...)
	return nil
}

func (r *customRuleSetRemote) RuleSetDelete(_ context.Context, id string) error {
	r.deleted = append(r.deleted, id)
	return nil
}

func (r *customRuleSetRemote) RuleSetList(context.Context) ([]telemetry.RuleSetInfo, error) {
	list := []telemetry.RuleSetInfo{{ID: "geosite-cn", Status: "ready"}}
	for _, s := range r.synced {
		list = append(list, telemetry.RuleSetInfo{ID: s.ID, Name: s.Name, URL: s.URL, Status: "missing", Custom: true, AutoUpdate: s.AutoUpdate, UpdateHours: s.UpdateHours})
	}
	return list, nil
}

func TestCustomRuleSetsLifecycleAndScheduledSync(t *testing.T) {
	s, _ := emptySession()
	remote := &customRuleSetRemote{}
	s.remote = remote
	ctx := context.Background()

	rs := spec.RuleSet{ID: "openai", Name: "OpenAI", URL: "https://example.com/openai.srs", AutoUpdate: true, UpdateHours: 6}
	saved, err := s.RuleSets().Save(ctx, rs)
	if err != nil || saved.ID != "openai" {
		t.Fatalf("Save failed: %v", err)
	}
	if _, err := s.Rules().Save(ctx, spec.Rule{Name: "AI", Enabled: true, Hosts: []string{"ruleset:openai"}, Network: "any", Action: "proxy", TargetID: "proxy"}); err != nil {
		t.Fatalf("rule referencing custom set failed: %v", err)
	}
	if err := s.RuleSets().Remove(ctx, "openai"); err == nil {
		t.Fatal("allowed removing referenced custom rule-set")
	}
	now := time.Now()
	s.syncRuleSetsIfDue(ctx, now)
	if len(remote.updated) != 1 || remote.updated[0] != "openai" {
		t.Fatalf("expected custom rule-set scheduled sync, got %v", remote.updated)
	}
	s.syncRuleSetsIfDue(ctx, now.Add(3*time.Hour))
	if len(remote.updated) != 1 {
		t.Fatalf("expected 6h cooldown, got %v", remote.updated)
	}
	s.syncRuleSetsIfDue(ctx, now.Add(7*time.Hour))
	if len(remote.updated) != 2 {
		t.Fatalf("expected second sync after 7h, got %v", remote.updated)
	}
}

func TestAppGroupAddAppAndLazyPresetAndSplitDNSSettings(t *testing.T) {
	s, _ := emptySession()
	remote := &customRuleSetRemote{}
	s.remote = remote
	ctx := context.Background()

	opts := routing.DefaultLazyPresetOptions()
	opts.IntranetDNS = "udp://10.0.0.53"
	res, err := s.Rules().ApplyLazyPreset(ctx, opts)
	if err != nil {
		t.Fatalf("ApplyLazyPreset failed: %v", err)
	}
	if res.RuleSetCount < 5 || res.AppGroupCount < 3 || res.DNSRuleCount != 2 {
		t.Fatalf("unexpected preset summary: %+v", res)
	}
	updated, err := s.AppGroups().AddApp(ctx, "ag-ai-dev", "Windsurf.exe")
	if err != nil {
		t.Fatalf("AddApp failed: %v", err)
	}
	if len(updated.Apps) == 0 || updated.Apps[len(updated.Apps)-1] != "Windsurf.exe" {
		t.Fatalf("AddApp did not append app: %+v", updated)
	}
	_, _, settingsSvc := s.Services()
	got, err := settingsSvc.Get(ctx)
	if err != nil || len(got.DNS.Servers) != 1 || len(got.DNS.Rules) != 2 || !got.RuleSetAutoUpdate {
		t.Fatalf("settings did not reflect preset split DNS: %+v %v", got, err)
	}
	got.RuleSetUpdateHours = 12
	if err := settingsSvc.Set(ctx, got); err != nil {
		t.Fatalf("Settings.Set with split DNS failed: %v", err)
	}
}
