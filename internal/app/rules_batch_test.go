package app

import (
	"context"
	"testing"

	"ferry/internal/spec"
)

func TestRulesSaveManyIsOneTransaction(t *testing.T) {
	s, _ := configured(t)
	rules := s.Rules()
	ctx := context.Background()
	before := s.revision

	batch := []spec.Rule{newRule("甲"), newRule("乙"), newRule("丙")}
	batch[1].Apps = nil
	batch[2].ID = "preview-3" // 预览里的临时 id 不会被沿用
	if err := rules.SaveMany(ctx, batch); err != nil {
		t.Fatal(err)
	}
	if s.revision != before+1 {
		t.Fatalf("batch must apply once: revision %d -> %d", before, s.revision)
	}
	list, _ := rules.List(ctx)
	if len(list) != 5 || !list[0].Pinned || !list[4].Pinned {
		t.Fatalf("pinned rules must stay at both ends: %+v", list)
	}
	seen := map[string]bool{}
	for i, name := range []string{"甲", "乙", "丙"} {
		rule := list[i+1]
		if rule.Name != name || rule.ID == "" || rule.ID == "preview-3" || seen[rule.ID] || rule.Apps == nil {
			t.Fatalf("rule %d: %+v", i, rule)
		}
		seen[rule.ID] = true
	}
}

func TestRulesSaveManyRejectsWholeBatch(t *testing.T) {
	s, _ := configured(t)
	rules := s.Rules()
	ctx := context.Background()

	bad := newRule("坏")
	bad.Ports = []string{"0"}
	pinned := newRule("固定")
	pinned.Pinned = true
	cases := map[string][]spec.Rule{
		"empty":   {},
		"invalid": {newRule("好"), bad},
		"pinned":  {newRule("好"), pinned},
		"builtin": {{ID: "builtin-default", Name: "x"}},
		"toomany": make([]spec.Rule, spec.MaxRules+1),
	}
	for name, batch := range cases {
		if err := rules.SaveMany(ctx, batch); err == nil {
			t.Fatalf("%s: expected an error", name)
		}
		if list, _ := rules.List(ctx); len(list) != 2 {
			t.Fatalf("%s: a rejected batch must write nothing, got %d rules", name, len(list))
		}
	}
}
