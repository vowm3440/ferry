package app

import (
	"context"
	"errors"
	"strings"
	"testing"

	"ferry/internal/spec"
)

func TestAppGroupsUpdateRunningConfigAndEmitAppliedVersion(t *testing.T) {
	s, remote := configured(t)
	ctx := context.Background()
	group, err := s.AppGroups().Save(ctx, spec.AppGroup{Name: "浏览器", Apps: []string{"chrome.exe"}})
	if err != nil {
		t.Fatal(err)
	}
	rule := newRule("引用分组")
	rule.Apps = []string{"@group:" + group.ID}
	if _, err := s.Rules().Save(ctx, rule); err != nil {
		t.Fatal(err)
	}
	if err := (&Core{s}).Start(ctx); err != nil {
		t.Fatal(err)
	}
	version, appliedVersion, events := s.revision, uint64(0), 0
	s.SetEmitter(func(event string, payload any) {
		if event == "rules:applied" {
			events++
			appliedVersion = payload.(map[string]uint64)["version"]
			_, _ = s.AppGroups().List(ctx)
		}
	})
	group.Apps = []string{"firefox.exe"}
	if _, err := s.AppGroups().Save(ctx, group); err != nil {
		t.Fatal(err)
	}
	live, err := spec.Decode(remote.data)
	if err != nil || len(live.AppGroups) != 1 || live.AppGroups[0].Apps[0] != "firefox.exe" {
		t.Fatalf("updated group not sent to running core: %v", err)
	}
	if live.Rules[1].Apps[0] != "@group:"+group.ID || remote.starts != 2 {
		t.Fatal("group update lost its reference or failed to restart core")
	}
	if events != 1 || appliedVersion != version+1 || s.revision != appliedVersion {
		t.Fatal("applied event did not reflect the committed revision")
	}
}

func TestAppGroupsFailedLiveSaveRestoresPreviousGroup(t *testing.T) {
	s, remote := configured(t)
	ctx := context.Background()
	group, err := s.AppGroups().Save(ctx, spec.AppGroup{Name: "浏览器", Apps: []string{"chrome.exe"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := (&Core{s}).Start(ctx); err != nil {
		t.Fatal(err)
	}
	s.storage = &rulesStorage{saveErr: errors.New("SECRET disk failure")}
	group.Apps = []string{"firefox.exe"}
	if _, err := s.AppGroups().Save(ctx, group); err == nil || strings.Contains(err.Error(), "SECRET") {
		t.Fatalf("failed update reported success or leaked error: %v", err)
	}
	live, err := spec.Decode(remote.data)
	if err != nil || live.AppGroups[0].Apps[0] != "chrome.exe" || remote.starts != 3 {
		t.Fatalf("core did not restore old group: starts=%d, %v", remote.starts, err)
	}
	groups, _ := s.AppGroups().List(ctx)
	if groups[0].Apps[0] != "chrome.exe" {
		t.Fatal("failed update changed session group")
	}
}
