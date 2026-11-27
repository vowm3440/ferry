package compile

import (
	"ferry/internal/routing"
	"ferry/internal/spec"
)

func compileRoute(s spec.RunSpec) (map[string]any, error) {
	rules := []any{
		map[string]any{"action": "sniff"},
		map[string]any{"protocol": []string{"dns"}, "action": "hijack-dns"},
		map[string]any{"clash_mode": "direct", "action": "route", "outbound": "__direct"},
		map[string]any{"clash_mode": "global", "action": "route", "outbound": defaultProxy(s)},
	}
	for _, r := range orderedRules(s.Rules) {
		if !r.Enabled {
			continue
		}
		resolve, err := routing.BuildResolve(s, r)
		if err != nil {
			return nil, err
		}
		if resolve != nil {
			rules = append(rules, resolve)
		}
		compiled, err := routing.Build(s, r)
		if err != nil {
			return nil, err
		}
		rules = append(rules, compiled)
	}
	route := map[string]any{"final": defaultProxy(s), "default_domain_resolver": "__dns_direct", "rules": rules}
	if s.Inbound.Tun.Enabled {
		route["auto_detect_interface"] = true
	}
	return route, nil
}

// Old saved configurations may omit one or both defaults. Validation guarantees
// any present defaults are unmodified; synthesize each exactly once here.
func orderedRules(rules []spec.Rule) []spec.Rule {
	defaults := spec.DefaultRules()
	result := make([]spec.Rule, 0, len(rules)+2)
	result = append(result, defaults[0])
	for _, r := range rules {
		if r.ID != spec.BuiltinLANRuleID && r.ID != spec.BuiltinDefaultRuleID {
			result = append(result, r)
		}
	}
	return append(result, defaults[1])
}
