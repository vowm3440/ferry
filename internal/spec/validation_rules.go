package spec

import (
	"errors"
	"net/netip"
	"slices"
	"strconv"
	"strings"
	"unicode"
)

const (
	BuiltinLANRuleID     = "builtin-lan"
	BuiltinDefaultRuleID = "builtin-default"
)

// DefaultRules returns new slices so editing a draft cannot alter the defaults.
func DefaultRules() []Rule {
	return []Rule{
		{ID: BuiltinLANRuleID, Name: "本机 / 局域网", Enabled: true, Pinned: true, Network: "any", Action: "direct",
			Apps: []string{}, Ports: []string{}, Hosts: []string{"0.0.0.0/8", "10.0.0.0/8", "127.0.0.0/8", "169.254.0.0/16",
				"172.16.0.0/12", "192.168.0.0/16", "224.0.0.0/4", "255.255.255.255/32", "::/128", "::1/128", "fc00::/7", "fe80::/10", "ff00::/8"}},
		{ID: BuiltinDefaultRuleID, Name: "默认规则", Enabled: true, Pinned: true, Network: "any", Action: "proxy", TargetID: "proxy",
			Apps: []string{}, Hosts: []string{}, Ports: []string{}},
	}
}

func (s RunSpec) validateRules() error {
	if len(s.Rules) > MaxRules {
		return errors.New("规则数量超过限制")
	}
	seen := make(map[string]bool)
	for i, r := range s.Rules {
		if seen[r.ID] {
			return errors.New("规则标识重复")
		}
		seen[r.ID] = true
		if r.ID == BuiltinLANRuleID && i != 0 || r.ID == BuiltinDefaultRuleID && i != len(s.Rules)-1 {
			return errors.New("固定规则必须位于首条和末条")
		}
		if err := s.ValidateRule(r); err != nil {
			return err
		}
	}
	return s.validateAppExpansion()
}

// ValidateRule validates one draft with an assigned ID, independently of its position.
func (s RunSpec) ValidateRule(r Rule) error {
	if !validID(r.ID) || !validLabel(r.Name) {
		return errors.New("规则标识或名称无效")
	}
	if !oneOf(r.Network, "any", "tcp", "udp") || !oneOf(r.Action, "direct", "block", "proxy") {
		return errors.New("规则协议或动作无效")
	}
	if err := validatePinnedRule(r); err != nil {
		return err
	}
	if !s.validRuleTarget(r) {
		return errors.New("规则目标不存在或不适用于此动作")
	}
	if len(r.Apps) > MaxRuleItems || len(r.Hosts) > MaxRuleItems || len(r.Ports) > MaxRuleItems {
		return errors.New("规则条件数量超过限制")
	}
	if _, err := s.ExpandApps(r.Apps); err != nil {
		return err
	}
	for _, host := range r.Hosts {
		if _, ok := RuleSetID(host); ok {
			continue
		}
		if strings.HasPrefix(strings.ToLower(host), "geoip:") || strings.HasPrefix(strings.ToLower(host), "geosite:") {
			return errors.New("规则集不在支持的目录内，请选择已列出的规则集")
		}
		if !validRuleHost(host) {
			return errors.New("规则主机条件无效或语法不受支持")
		}
	}
	for _, port := range r.Ports {
		if !validRulePort(port) {
			return errors.New("规则端口必须为 1 到 65535 或递增的端口范围")
		}
	}
	return nil
}

func validatePinnedRule(r Rule) error {
	for _, builtin := range DefaultRules() {
		if r.ID != builtin.ID {
			continue
		}
		if r.Name == builtin.Name && r.Enabled && r.Pinned && r.Network == builtin.Network &&
			r.Action == builtin.Action && r.TargetID == builtin.TargetID && r.ResolveBefore == builtin.ResolveBefore &&
			slices.Equal(r.Apps, builtin.Apps) && slices.Equal(r.Hosts, builtin.Hosts) && slices.Equal(r.Ports, builtin.Ports) {
			return nil
		}
		return errors.New("固定规则不可修改")
	}
	if r.Pinned {
		return errors.New("普通规则不可标记为固定规则")
	}
	return nil
}

func (s RunSpec) validRuleTarget(r Rule) bool {
	if r.Action != "proxy" {
		return r.TargetID == ""
	}
	if r.TargetID == "" || r.TargetID == "proxy" {
		return true
	}
	for _, node := range s.Nodes {
		if node.ID == r.TargetID {
			return true
		}
	}
	for _, group := range s.Groups {
		if group.ID == r.TargetID {
			return true
		}
	}
	return false
}

func validRuleText(value string, limit int) bool {
	return value != "" && len(value) <= limit && strings.TrimSpace(value) == value && !strings.ContainsFunc(value, unicode.IsControl)
}

func validRuleApp(app string) bool {
	if !validRuleText(app, 1024) || strings.ContainsAny(app, "\"<>|") {
		return false
	}
	if !strings.Contains(app, ":") {
		return true
	}
	// A colon is only meaningful as part of a fully qualified drive path.
	return len(app) > 3 && (app[0] >= 'A' && app[0] <= 'Z' || app[0] >= 'a' && app[0] <= 'z') &&
		app[1] == ':' && (app[2] == '\\' || app[2] == '/') && !strings.Contains(app[2:], ":")
}

func validRuleHost(host string) bool {
	if !validRuleText(host, 253) {
		return false
	}
	if address, err := netip.ParseAddr(host); err == nil {
		return address.Zone() == "" && !address.Is4In6()
	}
	if strings.Contains(host, "/") {
		prefix, err := netip.ParsePrefix(host)
		return err == nil && !prefix.Addr().Is4In6()
	}
	if start, end, found := strings.Cut(host, "-"); found {
		first, errFirst := netip.ParseAddr(start)
		last, errLast := netip.ParseAddr(end)
		if errFirst == nil || errLast == nil || strings.Trim(host, "0123456789.-") == "" {
			return errFirst == nil && errLast == nil && first.Zone() == "" && last.Zone() == "" &&
				first.BitLen() == last.BitLen() && !first.Is4In6() && !last.Is4In6() && first.Compare(last) <= 0
		}
	}
	return validRuleDomain(host)
}

func validRuleDomain(host string) bool {
	if strings.Trim(host, "0123456789.") == "" {
		return false
	}
	for _, label := range strings.Split(strings.TrimSuffix(host, "."), ".") {
		if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, c := range label {
			if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_' || c == '*' || c == '?') {
				return false
			}
		}
	}
	return true
}

func validRulePort(value string) bool {
	parts := strings.Split(value, "-")
	if len(parts) > 2 {
		return false
	}
	previous := uint64(0)
	for _, part := range parts {
		port, err := strconv.ParseUint(part, 10, 16)
		if err != nil || port == 0 || port < previous {
			return false
		}
		previous = port
	}
	return true
}
