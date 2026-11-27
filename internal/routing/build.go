// Package routing builds portable rule JSON without linking the core into the GUI.
package routing

import (
	"ferry/internal/spec"
	"strconv"
	"strings"
)

// Build validates and compiles one rule. The caller controls enabled/order semantics.
func Build(s spec.RunSpec, r spec.Rule) (map[string]any, error) {
	if err := s.ValidateRule(r); err != nil {
		return nil, err
	}
	apps, err := s.ExpandApps(r.Apps)
	if err != nil {
		return nil, err
	}
	r.Apps = apps
	conditions := otherConditions(r)
	if hosts := hostConditions(r.Hosts); len(hosts) > 0 {
		conditions = append(conditions, logical("or", hosts))
	}
	result := logical("and", conditions)
	result["action"] = "route"
	switch r.Action {
	case "block":
		result["action"] = "reject"
	case "direct":
		result["outbound"] = "__direct"
	case "proxy":
		result["outbound"] = OutboundTag(s, r.TargetID)
		if r.ID == spec.BuiltinDefaultRuleID {
			result["outbound"] = OutboundTag(s, "")
		}
	}
	return result, nil
}

// OutboundTag maps empty/default proxy targets to the actual selector tag.
// Existing groups named "proxy" remain explicitly addressable by their ID.
// Explicit targets must have passed RunSpec.ValidateRule first.
func OutboundTag(s spec.RunSpec, target string) string {
	if target == "proxy" {
		for _, group := range s.Groups {
			if group.ID == target {
				return target
			}
		}
	}
	if target != "" && target != "proxy" {
		return target
	}
	if len(s.Groups) > 0 {
		return s.Groups[0].ID
	}
	return "proxy"
}

func otherConditions(r spec.Rule) []map[string]any {
	conditions := make([]map[string]any, 0, 4)
	if len(r.Apps) > 0 {
		expressions := make([]string, 0, len(r.Apps))
		for _, app := range r.Apps {
			expressions = append(expressions, processPattern(app))
		}
		conditions = append(conditions, map[string]any{"process_path_regex": expressions})
	}
	if r.Network != "any" {
		conditions = append(conditions, map[string]any{"network": []string{r.Network}})
	}
	if len(r.Ports) > 0 {
		ports, ranges := []uint16{}, []string{}
		for _, port := range r.Ports {
			if strings.Contains(port, "-") {
				ranges = append(ranges, strings.ReplaceAll(port, "-", ":"))
				continue
			}
			value, _ := strconv.ParseUint(port, 10, 16)
			ports = append(ports, uint16(value))
		}
		fields := []map[string]any{}
		if len(ports) > 0 {
			fields = append(fields, map[string]any{"port": ports})
		}
		if len(ranges) > 0 {
			fields = append(fields, map[string]any{"port_range": ranges})
		}
		conditions = append(conditions, logical("or", fields))
	}
	return conditions
}

func logical(mode string, rules []map[string]any) map[string]any {
	if len(rules) == 0 {
		return map[string]any{}
	}
	if len(rules) == 1 {
		return rules[0]
	}
	return map[string]any{"type": "logical", "mode": mode, "rules": rules}
}
