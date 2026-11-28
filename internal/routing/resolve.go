package routing

import (
	"ferry/internal/spec"
	"strings"
)

// NeedsResolve reports whether the enabled rule requests real IP resolution.
func NeedsResolve(r spec.Rule) bool {
	if !r.Enabled || !r.ResolveBefore {
		return false
	}
	for _, host := range r.Hosts {
		if len(ipPrefixes(host)) > 0 || strings.HasPrefix(host, "geoip:") {
			return true
		}
	}
	return false
}

// BuildResolve preserves non-IP preconditions and skips an already matched domain
// alternative. A fixed real DNS server prevents fake-IP DNS rules from resolving
// the domain back to another fake address. Mode overrides precede this rule.
func BuildResolve(s spec.RunSpec, r spec.Rule) (map[string]any, error) {
	if err := s.ValidateRule(r); err != nil {
		return nil, err
	}
	if !NeedsResolve(r) {
		return nil, nil
	}
	apps, err := s.ExpandApps(r.Apps)
	if err != nil {
		return nil, err
	}
	r.Apps = apps
	conditions := otherConditions(r)
	domains := []string{}
	for _, host := range r.Hosts {
		if len(ipPrefixes(host)) == 0 && !strings.HasPrefix(host, "geoip:") {
			domains = append(domains, host)
		}
	}
	if len(domains) > 0 {
		notMatched := logical("or", hostConditions(s, domains))
		notMatched["invert"] = true
		conditions = append(conditions, notMatched)
	}
	result := logical("and", conditions)
	result["action"], result["server"], result["strategy"] = "resolve", "__dns_remote", s.DNS.Strategy
	if r.Action == "direct" {
		result["server"] = "__dns_direct"
	}
	return result, nil
}
