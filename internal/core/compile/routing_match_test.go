package compile

import (
	"context"
	"testing"

	"ferry/internal/spec"
	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/experimental/clashmode"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	sbrule "github.com/sagernet/sing-box/route/rule"
	M "github.com/sagernet/sing/common/metadata"
	"github.com/sagernet/sing/service"
)

func matchingAction(t *testing.T, options []option.Rule, mode, host string, port uint16) (string, string) {
	t.Helper()
	ctx := service.ExtendContext(context.Background())
	logger := log.NewNOPFactory().NewLogger("test")
	manager := clashmode.NewManager(ctx, logger, mode, []string{"rule", "global", "direct"})
	ctx = service.ContextWithPtr(ctx, manager)
	metadata := &adapter.InboundContext{Network: "tcp", Domain: host, Destination: M.ParseSocksaddrHostPort(host, port)}
	for _, options := range options {
		rule, err := sbrule.NewRule(ctx, logger, options, false)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = rule.Close() })
		if err := rule.Start(); err != nil {
			t.Fatal(err)
		}
		metadata.ResetRuleCache()
		if !rule.Match(metadata) {
			continue
		}
		if route, ok := rule.Action().(*sbrule.RuleActionRoute); ok {
			return "route", route.Outbound
		}
		if action := rule.Action().Type(); action == "reject" || action == "resolve" {
			return action, ""
		}
	}
	return "", ""
}

func TestModeOverridesAndFirstMatchUseSingboxEngine(t *testing.T) {
	s := compileRuleFixture()
	s.Rules = append([]spec.Rule{{ID: "all", Name: "Block all", Enabled: true, Network: "any", Action: "block"}}, s.Rules...)
	_, options := compiledRouteMaps(t, s)
	for _, tc := range []struct{ mode, host, action, tag string }{
		{"rule", "192.168.1.2", "route", "__direct"},
		{"rule", "example.com", "reject", ""},
		{"rule", "203.0.113.7", "reject", ""},
		{"global", "192.168.1.2", "route", "proxy"},
		{"global", "example.com", "route", "proxy"},
		{"direct", "example.com", "route", "__direct"},
	} {
		action, tag := matchingAction(t, options.Route.Rules, tc.mode, tc.host, 443)
		if action != tc.action || tag != tc.tag {
			t.Errorf("mode %s host %s: %s/%s, want %s/%s", tc.mode, tc.host, action, tag, tc.action, tc.tag)
		}
	}
}

func TestUnmatchedRulesReachDefaultAndBypassResolve(t *testing.T) {
	s := compileRuleFixture()
	s.Rules[1].Ports = []string{"80"}
	_, options := compiledRouteMaps(t, s)
	for _, tc := range []struct{ mode, action, tag string }{
		{"rule", "route", "proxy"}, {"global", "route", "proxy"}, {"direct", "route", "__direct"},
	} {
		action, tag := matchingAction(t, options.Route.Rules, tc.mode, "other.example", 443)
		if action != tc.action || tag != tc.tag {
			t.Errorf("%s: %s/%s", tc.mode, action, tag)
		}
	}
}
