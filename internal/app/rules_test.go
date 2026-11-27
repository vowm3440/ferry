package app

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"ferry/internal/spec"
)

type rulesStorage struct {
	data    []byte
	saveErr error
}

func TestRuleDraftOmittedArraysRemainValidAcrossIPC(t *testing.T) {
	s, _ := configured(t)
	ctx := context.Background()
	draft := newRule("draft")
	draft.Apps = nil
	draft.Ports = nil
	if _, err := s.Rules().Save(ctx, draft); err != nil {
		t.Fatal(err)
	}
	if err := (&Core{s}).Start(ctx); err != nil {
		t.Fatalf("saved rule cannot pass core wire validation: %v", err)
	}
}

func (s *rulesStorage) Load() ([]byte, error) { return append([]byte{}, s.data...), nil }
func (s *rulesStorage) Save(data []byte) error {
	if s.saveErr != nil {
		return s.saveErr
	}
	s.data = append([]byte{}, data...)
	return nil
}

func newRule(name string) spec.Rule {
	return spec.Rule{Name: name, Enabled: true, Apps: []string{}, Hosts: []string{"*.example.com"}, Ports: []string{"443"}, Network: "tcp", Action: "direct", ResolveBefore: true}
}

func TestRulesPreservePinnedOrderAndRejectInvalidEdits(t *testing.T) {
	s, _ := configured(t)
	rules := s.Rules()
	ctx := context.Background()
	initial, err := rules.List(ctx)
	if err != nil || len(initial) != 2 || !initial[0].Pinned || !initial[1].Pinned {
		t.Fatalf("defaults: %+v %v", initial, err)
	}
	first, err := rules.Save(ctx, newRule("浏览器"))
	if err != nil {
		t.Fatal(err)
	}
	second, err := rules.Save(ctx, newRule("工作"))
	if err != nil {
		t.Fatal(err)
	}
	if first.ID == second.ID || first.ID == "" {
		t.Fatal("rule IDs not unique")
	}
	if err = rules.Reorder(ctx, []string{initial[0].ID, second.ID, first.ID, initial[1].ID}); err != nil {
		t.Fatal(err)
	}
	list, _ := rules.List(ctx)
	if list[1].ID != second.ID {
		t.Fatal("reorder not applied")
	}
	list[1].Hosts[0] = "changed"
	list, _ = rules.List(ctx)
	if list[1].Hosts[0] != "*.example.com" {
		t.Fatal("caller can mutate saved rules")
	}
	if err = rules.Reorder(ctx, []string{initial[1].ID, second.ID, first.ID, initial[0].ID}); err == nil {
		t.Fatal("pinned rules moved")
	}
	if err = rules.Remove(ctx, initial[0].ID); err == nil {
		t.Fatal("pinned rule removed")
	}
	if err = rules.SetEnabled(ctx, initial[1].ID, false); err == nil {
		t.Fatal("pinned rule disabled")
	}
	if _, err = rules.Save(ctx, initial[0]); err == nil {
		t.Fatal("pinned rule edited")
	}
	bad := newRule("bad")
	bad.Action = "proxy"
	bad.TargetID = "missing"
	if _, err = rules.Save(ctx, bad); err == nil {
		t.Fatal("invalid target accepted")
	}
	if err = rules.Remove(ctx, "missing"); err == nil {
		t.Fatal("missing rule accepted")
	}
	if err = rules.SetEnabled(ctx, first.ID, false); err != nil {
		t.Fatal(err)
	}
	if err = rules.Remove(ctx, second.ID); err != nil {
		t.Fatal(err)
	}
	list, _ = rules.List(ctx)
	if len(list) != 3 || list[1].Enabled {
		t.Fatal("disable/remove not applied")
	}
}

func TestRulesPersistAndRollbackOnSaveFailure(t *testing.T) {
	s, _ := configured(t)
	storage := &rulesStorage{}
	s.storage = storage
	ctx := context.Background()
	saved, err := s.Rules().Save(ctx, newRule("持久化"))
	if err != nil {
		t.Fatal(err)
	}
	reloaded, err := NewPersistentSession(s.remote, storage)
	if err != nil {
		t.Fatal(err)
	}
	list, _ := reloaded.Rules().List(ctx)
	if len(list) != 3 || list[1].ID != saved.ID {
		t.Fatal("rule not persisted")
	}
	storage.saveErr = errors.New("disk failure")
	before, _ := s.Rules().List(ctx)
	if err = s.Rules().Remove(ctx, saved.ID); err == nil {
		t.Fatal("failed save reported success")
	}
	after, _ := s.Rules().List(ctx)
	if !reflect.DeepEqual(before, after) {
		t.Fatal("failed write changed rules")
	}
}
