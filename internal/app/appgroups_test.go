package app

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"ferry/internal/spec"
)

func TestAppGroupsPersistWithoutAliasingCallerState(t *testing.T) {
	s, _ := configured(t)
	storage := &rulesStorage{}
	s.storage = storage
	ctx := context.Background()
	draft := spec.AppGroup{Name: "浏览器", Apps: []string{"chrome.exe"}}
	saved, err := s.AppGroups().Save(ctx, draft)
	if err != nil || saved.ID == "" {
		t.Fatalf("save: %+v, %v", saved, err)
	}
	draft.Apps[0], saved.Apps[0] = "input-tampered.exe", "output-tampered.exe"
	list, err := s.AppGroups().List(ctx)
	if err != nil || len(list) != 1 || list[0].Apps[0] != "chrome.exe" {
		t.Fatalf("caller changed stored group: %+v, %v", list, err)
	}
	list[0].Apps[0] = "list-tampered.exe"
	again, _ := s.AppGroups().List(ctx)
	if again[0].Apps[0] != "chrome.exe" {
		t.Fatal("list shares members with session")
	}
	restored, err := NewPersistentSession(s.remote, storage)
	if err != nil {
		t.Fatal(err)
	}
	reloaded, err := restored.AppGroups().List(ctx)
	if err != nil || !reflect.DeepEqual(again, reloaded) {
		t.Fatalf("group did not persist: %+v, %v", reloaded, err)
	}
}

func TestAppGroupsOldSessionAndEmptyGroupsRemainCompatible(t *testing.T) {
	s, _ := configured(t)
	ctx := context.Background()
	groups, err := s.AppGroups().List(ctx)
	if err != nil || groups == nil || len(groups) != 0 {
		t.Fatalf("legacy groups: %+v, %v", groups, err)
	}
	first, err := s.AppGroups().Save(ctx, spec.AppGroup{Name: "空分组"})
	if err != nil || first.Apps == nil {
		t.Fatalf("empty group: %+v, %v", first, err)
	}
	second, err := s.AppGroups().Save(ctx, spec.AppGroup{Name: "第二组"})
	if err != nil || second.ID == first.ID {
		t.Fatalf("independent group ID: %+v, %v", second, err)
	}
	if err := (&Core{s}).Start(ctx); err != nil {
		t.Fatalf("empty group broke wire validation: %v", err)
	}
	if err := s.AppGroups().Remove(ctx, first.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.AppGroups().Remove(ctx, "missing"); err == nil {
		t.Fatal("missing group removal succeeded")
	}
}

func TestAppGroupsRefuseReferencedRemovalAndEmptyUpdate(t *testing.T) {
	s, _ := configured(t)
	ctx := context.Background()
	group, err := s.AppGroups().Save(ctx, spec.AppGroup{Name: "浏览器", Apps: []string{"chrome.exe"}})
	if err != nil {
		t.Fatal(err)
	}
	rule := newRule("引用分组")
	rule.Enabled, rule.Apps = false, []string{"@group:" + group.ID}
	savedRule, err := s.Rules().Save(ctx, rule)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.AppGroups().Remove(ctx, group.ID); err == nil {
		t.Fatal("removed group referenced by disabled rule")
	}
	group.Apps = nil
	if _, err := s.AppGroups().Save(ctx, group); err == nil {
		t.Fatal("emptied referenced group")
	}
	list, _ := s.AppGroups().List(ctx)
	if len(list) != 1 || list[0].Apps[0] != "chrome.exe" {
		t.Fatal("rejected mutation changed group")
	}
	if err := s.Rules().Remove(ctx, savedRule.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.AppGroups().Remove(ctx, group.ID); err != nil {
		t.Fatal(err)
	}
	list, _ = s.AppGroups().List(ctx)
	if len(list) != 0 {
		t.Fatal("unreferenced group was not removed")
	}
}

func TestAppGroupsFailedSaveLeavesMemoryStorageAndEventsUnchanged(t *testing.T) {
	s, _ := configured(t)
	storage := &rulesStorage{}
	s.storage = storage
	ctx := context.Background()
	group, err := s.AppGroups().Save(ctx, spec.AppGroup{Name: "浏览器", Apps: []string{"chrome.exe"}})
	if err != nil {
		t.Fatal(err)
	}
	before := string(storage.data)
	version := s.revision
	events := 0
	s.SetEmitter(func(event string, _ any) {
		if event == "rules:applied" {
			events++
		}
	})
	storage.saveErr = errors.New("SECRET disk failure")
	group.Apps[0] = "firefox.exe"
	if _, err := s.AppGroups().Save(ctx, group); err == nil || strings.Contains(err.Error(), "SECRET") {
		t.Fatalf("failed persistence reported success or leaked: %v", err)
	}
	list, _ := s.AppGroups().List(ctx)
	if list[0].Apps[0] != "chrome.exe" || string(storage.data) != before || s.revision != version || events != 0 {
		t.Fatal("failed transaction mutated groups, storage, version or events")
	}
}

func TestAppGroupsRejectMalformedDraftWithoutLeakingInput(t *testing.T) {
	s, _ := configured(t)
	ctx := context.Background()
	for _, group := range []spec.AppGroup{
		{Name: "SECRET\n", Apps: []string{"chrome.exe"}},
		{ID: "SECRET:invalid", Name: "无效", Apps: []string{"chrome.exe"}},
		{Name: "嵌套", Apps: []string{"@group:SECRET"}},
		{Name: "无效应用", Apps: []string{"regex:SECRET"}},
	} {
		if _, err := s.AppGroups().Save(ctx, group); err == nil || strings.Contains(err.Error(), "SECRET") {
			t.Fatalf("invalid group accepted or leaked in error: %v", err)
		}
	}
	list, _ := s.AppGroups().List(ctx)
	if len(list) != 0 {
		t.Fatal("invalid drafts changed groups")
	}
}
