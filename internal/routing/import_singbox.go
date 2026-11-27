package routing

import (
	"fmt"
	"net/netip"
	"strings"

	"ferry/internal/spec"
)

func importSingbox(content string, s spec.RunSpec) ([]importedRule, []string, error) {
	values, warnings, err := documentRuleList(content, true)
	if err != nil {
		return nil, nil, err
	}
	result := make([]importedRule, 0, len(values))
	for _, value := range values {
		object, ok := value.(map[string]any)
		if !ok {
			result = append(result, importedRule{reason: "规则必须为对象"})
			continue
		}
		result = append(result, singboxRule(object, s))
	}
	return result, warnings, nil
}

func singboxRule(object map[string]any, s spec.RunSpec) importedRule {
	r := importBaseRule()
	if kind, exists := object["type"]; exists && kind != "default" {
		return importedRule{reason: "逻辑规则或规则类型暂不支持"}
	}
	if _, name := object["process_name"]; name {
		if _, path := object["process_path"]; path {
			return importedRule{reason: "进程名称与路径组合无法保留且条件"}
		}
	}
	for key := range object {
		switch key {
		case "type", "action", "outbound":
		case "domain", "domain_suffix", "domain_keyword", "rule_set", "ip_cidr", "process_name", "process_path", "network", "port", "port_range":
		default:
			return importedRule{reason: "包含不受支持的条件或动作参数"}
		}
	}
	for _, key := range []string{"domain", "domain_suffix", "domain_keyword", "rule_set", "ip_cidr", "process_name", "process_path", "network", "port", "port_range"} {
		if value, exists := object[key]; exists && !singboxCondition(key, value, &r) {
			return importedRule{reason: "条件字段无法完整转换"}
		}
	}
	if !singboxAction(object, s, &r) {
		return importedRule{reason: "动作或出站目标无法完整转换"}
	}
	result := importedRule{rule: r}
	if len(r.Apps) > 0 {
		result.notice = "进程匹配将使用 Ferry 的 Windows 语义，忽略大小写"
	}
	if len(r.Apps)+len(r.Hosts)+len(r.Ports) == 0 && r.Network == "any" {
		result.notice = catchAllNotice
	}
	return result
}

func singboxAction(object map[string]any, s spec.RunSpec, r *spec.Rule) bool {
	action := "route"
	if value, exists := object["action"]; exists {
		var ok bool
		action, ok = value.(string)
		if !ok {
			return false
		}
	}
	if action == "reject" {
		if _, exists := object["outbound"]; exists {
			return false
		}
		r.Action = "block"
		return true
	}
	if action != "route" {
		return false
	}
	outbound, ok := object["outbound"].(string)
	return ok && importAction(s, outbound, r)
}

func singboxCondition(key string, value any, r *spec.Rule) bool {
	if key == "port" {
		return singboxPorts(value, r)
	}
	values, ok := importStrings(value)
	if !ok {
		return false
	}
	if key == "network" {
		return singboxNetwork(values, r)
	}
	for _, text := range values {
		if !singboxConditionItem(key, text, r) {
			return false
		}
	}
	return true
}

func singboxConditionItem(key, text string, r *spec.Rule) bool {
	switch key {
	case "domain":
		if !importLiteralHost(text) {
			return false
		}
		r.Hosts = append(r.Hosts, text)
	case "domain_suffix":
		if !importLiteralHost(strings.TrimPrefix(text, ".")) {
			return false
		}
		if strings.HasPrefix(text, ".") {
			r.Hosts = append(r.Hosts, "*"+text)
		} else {
			r.Hosts = append(r.Hosts, text, "*."+text)
		}
	case "domain_keyword":
		if text == "" || strings.ContainsAny(text, "*?") {
			return false
		}
		r.Hosts = append(r.Hosts, "*"+text+"*")
	case "rule_set":
		ref, ok := catalogReferenceByID(text)
		if !ok {
			return false
		}
		r.Hosts = append(r.Hosts, ref)
	case "ip_cidr":
		if _, err := netip.ParsePrefix(text); err != nil {
			return false
		}
		r.Hosts = append(r.Hosts, text)
	case "port_range":
		port, ok := importPortRange(text)
		if !ok {
			return false
		}
		r.Ports = append(r.Ports, port)
	case "process_name", "process_path":
		if strings.ContainsAny(text, "*?") || key == "process_name" && strings.ContainsAny(text, `\/`) {
			return false
		}
		r.Apps = append(r.Apps, text)
	}
	return true
}

func singboxPorts(value any, r *spec.Rule) bool {
	values, ok := value.([]any)
	if !ok {
		values = []any{value}
	}
	for _, value := range values {
		port, ok := value.(int)
		if !ok || port < 1 || port > 65535 {
			return false
		}
		r.Ports = append(r.Ports, fmt.Sprint(port))
	}
	return true
}

func singboxNetwork(values []string, r *spec.Rule) bool {
	seen := map[string]bool{}
	for _, value := range values {
		if value != "tcp" && value != "udp" {
			return false
		}
		seen[value] = true
	}
	if len(seen) > 1 {
		return false
	}
	if len(seen) == 1 {
		for network := range seen {
			r.Network = network
		}
	}
	return true
}

func importPortRange(value string) (string, bool) {
	start, end, found := strings.Cut(value, ":")
	if !found || strings.ContainsAny(value, "-") {
		return "", false
	}
	if start == "" {
		start = "0"
	}
	if end == "" {
		end = "65535"
	}
	return start + "-" + end, true
}
