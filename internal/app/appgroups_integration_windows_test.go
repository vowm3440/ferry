//go:build windows

package app

import (
	"context"
	"testing"
	"time"

	"ferry/internal/spec"
)

func TestAppGroupsUpdateRealCoreRuleTester(t *testing.T) {
	s, process, port := importProcessSession(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if _, err := s.Subs().AddFromText(ctx, "本地占位节点", "socks5://127.0.0.1:1080"); err != nil {
		t.Fatal(err)
	}
	group, err := s.AppGroups().Save(ctx, spec.AppGroup{Name: "浏览器", Apps: []string{"chrome.exe"}})
	if err != nil {
		t.Fatal(err)
	}
	rule := spec.Rule{Name: "应用分组阻断", Enabled: true, Network: "any", Action: "block", Apps: []string{"@group:" + group.ID}}
	saved, err := s.Rules().Save(ctx, rule)
	if err != nil {
		t.Fatal(err)
	}
	if err := (&Core{s}).Start(ctx); err != nil {
		t.Fatal(err)
	}
	assertAppGroupRuleMatch(t, s, ctx, "chrome.exe", saved.ID)
	group.Apps = []string{"firefox.exe"}
	if _, err := s.AppGroups().Save(ctx, group); err != nil {
		t.Fatal(err)
	}
	assertAppGroupRuleMatch(t, s, ctx, "chrome.exe", spec.BuiltinDefaultRuleID)
	assertAppGroupRuleMatch(t, s, ctx, "firefox.exe", saved.ID)
	if err := (&Core{s}).Stop(ctx); err != nil {
		t.Fatal(err)
	}
	assertImportProcessClosed(t, process, port)
}

func assertAppGroupRuleMatch(t *testing.T, s *Session, ctx context.Context, app, id string) {
	t.Helper()
	result, err := s.Rules().Test(ctx, app, "203.0.113.42", 443, "tcp")
	if err != nil || result.MatchedRuleID != id {
		t.Fatalf("group rule match: %+v, %v", result, err)
	}
}
