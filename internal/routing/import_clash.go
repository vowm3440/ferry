package routing

import (
	"errors"
	"net/netip"
	"strings"

	"ferry/internal/spec"
)

func importClash(content string, s spec.RunSpec) ([]importedRule, []string, error) {
	var values []any
	warnings := []string{}
	structured := structuredClash(content)
	if structured {
		var err error
		values, warnings, err = documentRuleList(content, false)
		if err != nil {
			return nil, nil, err
		}
	} else {
		for _, line := range strings.Split(content, "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			values = append(values, line)
		}
	}
	if len(values) > spec.MaxRules {
		return nil, nil, errors.New("规则数量超过限制")
	}
	result := make([]importedRule, 0, len(values))
	for _, value := range values {
		line, ok := value.(string)
		if !ok {
			result = append(result, importedRule{reason: "Clash 规则必须为文本条目"})
			continue
		}
		result = append(result, clashRule(line, s))
	}
	return result, warnings, nil
}

func structuredClash(content string) bool {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		return strings.HasPrefix(line, "[") || strings.HasPrefix(line, "{") || strings.HasPrefix(line, "-") || strings.HasPrefix(line, "rules:") || !strings.Contains(line, ",")
	}
	return false
}

func clashRule(line string, s spec.RunSpec) importedRule {
	parts := strings.Split(line, ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	r := importBaseRule()
	if len(parts) < 2 {
		return importedRule{reason: "规则字段不足"}
	}
	if parts[0] == "MATCH" {
		if len(parts) != 2 || !importAction(s, parts[1], &r) {
			return importedRule{reason: "兜底规则参数或动作不受支持"}
		}
		return importedRule{rule: r, notice: catchAllNotice}
	}
	if len(parts) < 3 {
		return importedRule{reason: "规则字段不足"}
	}
	isIP := parts[0] == "IP-CIDR" || parts[0] == "IP-CIDR6" || parts[0] == "GEOIP"
	if len(parts) > 3 && !(len(parts) == 4 && isIP && parts[3] == "no-resolve") {
		return importedRule{reason: "含不支持的额外参数"}
	}
	if !importAction(s, parts[2], &r) {
		return importedRule{reason: "动作目标不存在或名称不唯一"}
	}
	r.ResolveBefore = isIP && len(parts) == 3
	if !clashCondition(parts[0], parts[1], &r) {
		return importedRule{reason: "规则类型或条件无法完整转换"}
	}
	return importedRule{rule: r}
}

func clashCondition(kind, value string, r *spec.Rule) bool {
	switch kind {
	case "GEOIP", "GEOSITE", "RULE-SET", "DOMAIN", "DOMAIN-SUFFIX", "DOMAIN-KEYWORD", "DOMAIN-WILDCARD", "IP-CIDR", "IP-CIDR6":
		return clashHostCondition(kind, value, r)
	case "DST-PORT":
		r.Ports = []string{value}
	case "PROCESS-NAME", "PROCESS-PATH":
		if strings.ContainsAny(value, "*?") || kind == "PROCESS-NAME" && strings.ContainsAny(value, `\/`) {
			return false
		}
		r.Apps = []string{value}
	case "PROCESS-NAME-WILDCARD", "PROCESS-PATH-WILDCARD":
		if value == "" || kind == "PROCESS-NAME-WILDCARD" && strings.ContainsAny(value, `\/`) {
			return false
		}
		r.Apps = []string{value}
	case "NETWORK":
		r.Network = strings.ToLower(value)
		return r.Network == "tcp" || r.Network == "udp"
	default:
		return false
	}
	return true
}

func clashHostCondition(kind, value string, r *spec.Rule) bool {
	switch kind {
	case "GEOIP", "GEOSITE":
		reference := strings.ToLower(kind) + ":" + value
		if _, ok := spec.RuleSetID(reference); !ok {
			return false
		}
		r.Hosts = []string{reference}
	case "RULE-SET":
		reference, ok := catalogReferenceByID(value)
		if !ok {
			return false
		}
		r.Hosts = []string{reference}
	case "DOMAIN":
		if !importLiteralHost(value) {
			return false
		}
		r.Hosts = []string{value}
	case "DOMAIN-SUFFIX":
		if !importLiteralHost(value) {
			return false
		}
		r.Hosts = []string{value, "*." + value}
	case "DOMAIN-KEYWORD":
		if value == "" || strings.ContainsAny(value, "*?") {
			return false
		}
		r.Hosts = []string{"*" + value + "*"}
	case "DOMAIN-WILDCARD":
		if value == "" || !strings.ContainsAny(value, "*?") {
			return false
		}
		r.Hosts = []string{value}
	case "IP-CIDR", "IP-CIDR6":
		prefix, err := netip.ParsePrefix(value)
		if err != nil || (kind == "IP-CIDR6") != prefix.Addr().Is6() {
			return false
		}
		r.Hosts = []string{value}
	}
	return true
}

func catalogReferenceByID(id string) (string, bool) {
	switch id {
	case "geosite-cn":
		return "geosite:cn", true
	case "geosite-non-cn":
		return "geosite:geolocation-!cn", true
	case "geoip-cn":
		return "geoip:cn", true
	case "geoip-private":
		return "geoip:private", true
	default:
		return "", false
	}
}
