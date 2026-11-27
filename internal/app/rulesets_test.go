package app

import (
	"context"
	"ferry/internal/telemetry"
	"testing"
	"time"
)

type catalogueRemote struct {
	remoteFixture
	updated []string
}

func (r *catalogueRemote) RuleSetList(context.Context) ([]telemetry.RuleSetInfo, error) {
	return []telemetry.RuleSetInfo{{ID: "geosite-cn", Status: "ready"}}, nil
}
func (r *catalogueRemote) RuleSetUpdate(_ context.Context, id string) error {
	r.updated = append(r.updated, id)
	return nil
}

func TestRuleSetBatchValidatesAllIDsBeforeMutations(t *testing.T) {
	s, _ := emptySession()
	r := &catalogueRemote{}
	s.remote = r
	ctx := context.Background()
	if err := s.RuleSets().Update(ctx, []string{"geosite-cn", "../escape"}); err == nil {
		t.Fatal("invalid catalogue ID accepted")
	}
	if len(r.updated) != 0 {
		t.Fatal("valid prefix was updated before full batch validation")
	}
	if err := s.RuleSets().Update(ctx, []string{"geosite-cn", "geosite-cn"}); err == nil {
		t.Fatal("duplicate accepted")
	}
	if err := s.RuleSets().Update(ctx, []string{"geosite-cn", "geoip-cn"}); err != nil {
		t.Fatal(err)
	}
	if len(r.updated) != 2 {
		t.Fatal("valid batch not forwarded")
	}
}

func TestSyncRuleSetsIfDueUsesRuleSetManager(t *testing.T) {
	s, _ := emptySession()
	r := &catalogueRemote{}
	s.remote = r
	s.settings.RuleSetAutoUpdate = true
	now := time.Now()
	s.syncRuleSetsIfDue(context.Background(), now)
	if len(r.updated) != 4 {
		t.Fatalf("expected all 4 catalog rule sets updated, got %d (%v)", len(r.updated), r.updated)
	}
	s.syncRuleSetsIfDue(context.Background(), now.Add(time.Hour))
	if len(r.updated) != 4 {
		t.Fatalf("expected 24h cooldown to skip duplicate sync, got %d", len(r.updated))
	}
}
