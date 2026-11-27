package compile

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	include "ferry/internal/core/boxinclude"
	"ferry/internal/spec"
	box "github.com/sagernet/sing-box"
	"github.com/sagernet/sing-box/option"
	corejson "github.com/sagernet/sing/common/json"
	"github.com/sagernet/sing/service"
)

func compileRuleFixture() spec.RunSpec {
	s := fixture()
	s.Rules = []spec.Rule{
		{ID: "one", Name: "First", Enabled: true, Network: "tcp", Action: "direct", Hosts: []string{"example.com"}},
		{ID: "two", Name: "Second", Enabled: true, Network: "any", Action: "block", Hosts: []string{"203.0.113.1-203.0.113.9"}, ResolveBefore: true},
		{ID: "disabled", Name: "Disabled", Enabled: false, Network: "any", Action: "block"},
	}
	return s
}

func compiledRouteMaps(t *testing.T, s spec.RunSpec) ([]map[string]any, option.Options) {
	t.Helper()
	options, err := Compile(s)
	if err != nil {
		t.Fatal(err)
	}
	data, err := corejson.Marshal(options.Route.Rules)
	if err != nil {
		t.Fatal(err)
	}
	var result []map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatal(err)
	}
	return result, options
}

func TestCompileOrderedRulesDefaultsAndResolve(t *testing.T) {
	rules, options := compiledRouteMaps(t, compileRuleFixture())
	if len(rules) != 9 {
		t.Fatalf("expected prelude, LAN, first, resolve, second, default; got %v", rules)
	}
	if rules[2]["clash_mode"] != "direct" || rules[3]["clash_mode"] != "global" {
		t.Fatal("mode override lost priority")
	}
	if rules[4]["outbound"] != "__direct" || rules[5]["outbound"] != "__direct" || rules[6]["action"] != "resolve" ||
		rules[6]["server"] != "__dns_remote" || rules[7]["action"] != "reject" || rules[8]["outbound"] != "proxy" {
		t.Fatalf("wrong order: %v", rules)
	}
	ctx := service.ExtendContext(include.Context(context.Background()))
	instance, err := box.New(box.Options{Context: ctx, Options: options})
	if err != nil {
		t.Fatalf("box rejected rules: %v", err)
	}
	if err := instance.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestExplicitDefaultsAreNotDuplicated(t *testing.T) {
	s := compileRuleFixture()
	legacy, _ := compiledRouteMaps(t, s)
	defaults := spec.DefaultRules()
	s.Rules = append(append([]spec.Rule{defaults[0]}, s.Rules...), defaults[1])
	explicit, _ := compiledRouteMaps(t, s)
	if !reflect.DeepEqual(explicit, legacy) {
		t.Fatalf("explicit defaults changed routing:\n%v\n%v", explicit, legacy)
	}
}

func TestCompileResolveUsesRealDNSForFakeIPAndDirectAction(t *testing.T) {
	s := compileRuleFixture()
	s.DNS.Mode = "fakeip"
	s.Rules = []spec.Rule{{ID: "r", Name: "Direct IP", Enabled: true, Network: "any", Action: "direct", Hosts: []string{"203.0.113.0/24"}, ResolveBefore: true}}
	rules, options := compiledRouteMaps(t, s)
	if len(rules) != 8 || rules[5]["action"] != "resolve" || rules[5]["server"] != "__dns_direct" {
		t.Fatalf("wrong direct DNS: %v", rules)
	}
	ctx := service.ExtendContext(include.Context(context.Background()))
	instance, err := box.New(box.Options{Context: ctx, Options: options})
	if err != nil {
		t.Fatal(err)
	}
	if err := instance.Close(); err != nil {
		t.Fatal(err)
	}
}
