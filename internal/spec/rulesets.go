package spec

import (
	"errors"
	"net/url"
	"strings"
	"unicode"
)

const CustomRuleSetPrefix = "ruleset:"

// RuleSetID maps built-in public rule grammar to fixed internal catalogue IDs.
func RuleSetID(reference string) (string, bool) {
	switch reference {
	case "geosite:cn":
		return "geosite-cn", true
	case "geosite:geolocation-!cn":
		return "geosite-non-cn", true
	case "geoip:cn":
		return "geoip-cn", true
	case "geoip:private":
		return "geoip-private", true
	default:
		return "", false
	}
}

func ValidRuleSetID(id string) bool {
	switch id {
	case "geosite-cn", "geosite-non-cn", "geoip-cn", "geoip-private":
		return true
	default:
		return false
	}
}

func RuleSetCatalogIDs() []string {
	return []string{"geosite-cn", "geosite-non-cn", "geoip-cn", "geoip-private"}
}

// ValidRuleSetURL requires a bounded HTTPS .srs address with no credentials or fragments.
func ValidRuleSetURL(raw string) bool {
	if raw == "" || len(raw) > 2048 || strings.TrimSpace(raw) != raw || strings.ContainsFunc(raw, unicode.IsControl) {
		return false
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.Fragment != "" {
		return false
	}
	return strings.HasSuffix(strings.ToLower(u.Path), ".srs") && !strings.ContainsAny(u.Host, "\\%")
}

// ValidateRuleSet checks a single custom remote SRS definition.
func ValidateRuleSet(rs RuleSet) error {
	if !validID(rs.ID) || ValidRuleSetID(rs.ID) || !validLabel(rs.Name) {
		return errors.New("自定义规则集标识或名称无效，且不能与内置规则集重名")
	}
	if !ValidRuleSetURL(rs.URL) {
		return errors.New("自定义规则集地址须为有效的 HTTPS .srs 链接")
	}
	if rs.UpdateHours < 0 || rs.UpdateHours > 720 {
		return errors.New("规则集刷新间隔须在 0 到 720 小时之间")
	}
	return nil
}

func (s RunSpec) validateRuleSets() error {
	if len(s.RuleSets) > MaxCustomRuleSets {
		return errors.New("自定义规则集数量超过限制")
	}
	seen := make(map[string]bool, len(s.RuleSets))
	for _, rs := range s.RuleSets {
		if seen[rs.ID] {
			return errors.New("自定义规则集标识重复")
		}
		if err := ValidateRuleSet(rs); err != nil {
			return err
		}
		seen[rs.ID] = true
	}
	return nil
}

// RuleSetID resolves both built-in (geosite:/geoip:) and declared custom (ruleset:<id>) references.
func (s RunSpec) RuleSetID(reference string) (string, bool) {
	if id, ok := RuleSetID(reference); ok {
		return id, true
	}
	id, ok := strings.CutPrefix(reference, CustomRuleSetPrefix)
	if !ok || !validID(id) {
		return "", false
	}
	for _, rs := range s.RuleSets {
		if rs.ID == id {
			return id, true
		}
	}
	return "", false
}

// HasRuleSet reports whether id is either a built-in catalog ID or a declared custom rule-set ID.
func (s RunSpec) HasRuleSet(id string) bool {
	if ValidRuleSetID(id) {
		return true
	}
	for _, rs := range s.RuleSets {
		if rs.ID == id {
			return true
		}
	}
	return false
}

// ReferencedRuleSets collects all built-in and custom rule-sets referenced by enabled route and DNS rules.
func (s RunSpec) ReferencedRuleSets() []string {
	result := []string{}
	seen := map[string]bool{}
	collect := func(items []string) {
		for _, item := range items {
			if id, ok := s.RuleSetID(item); ok && !seen[id] {
				seen[id] = true
				result = append(result, id)
			}
		}
	}
	for _, rule := range s.Rules {
		if rule.Enabled {
			collect(rule.Hosts)
		}
	}
	for _, rule := range s.DNS.Rules {
		if rule.Enabled {
			collect(rule.Domains)
		}
	}
	return result
}
