package spec

import (
	"errors"
	"net/netip"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
)

var idPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{0,63}$`)

func (s RunSpec) validateSettings() error {
	if err := validateTun(s.Inbound.Tun); err != nil {
		return err
	}
	if s.Inbound.MixedPort == 0 {
		return errors.New("混合代理端口必须在 1 到 65535 之间")
	}
	if !oneOf(s.Mode, "rule", "global", "direct") {
		return errors.New("代理模式无效")
	}
	if !oneOf(s.Log.Level, "trace", "debug", "info", "warn", "error", "fatal", "panic") {
		return errors.New("日志级别无效")
	}
	if !oneOf(s.DNS.Mode, "fakeip", "redirhost") || !oneOf(s.DNS.Strategy, "prefer_ipv4", "prefer_ipv6", "ipv4_only", "ipv6_only", "as_is") {
		return errors.New("DNS 模式或解析策略无效")
	}
	if !validDNS(s.DNS.Remote) || !validDNS(s.DNS.Direct) {
		return errors.New("DNS 仅支持使用 IP 地址的 HTTPS、TLS、TCP 或 UDP 服务器")
	}
	return nil
}

func validateTun(t Tun) error {
	if t.Stack != "" && !oneOf(t.Stack, "mixed", "system", "gvisor") {
		return errors.New("TUN 协议栈无效")
	}
	if t.MTU != 0 && (t.MTU < 1280 || t.MTU > 65535) {
		return errors.New("TUN MTU 必须在 1280 到 65535 之间")
	}
	if len(t.Excludes) > 256 {
		return errors.New("TUN 排除网段数量超过限制")
	}
	for _, entry := range t.Excludes {
		prefix, err := netip.ParsePrefix(entry)
		if err != nil || prefix.Addr().Zone() != "" {
			return errors.New("TUN 排除网段必须是有效的 IP CIDR")
		}
	}
	return nil
}

// DNS bootstrap uses a literal IP, avoiding accidental system DNS lookups.
func validDNS(value string) bool {
	u, err := url.Parse(value)
	if err != nil || len(value) > 2048 || u.User != nil || u.Fragment != "" || u.RawQuery != "" {
		return false
	}
	if !oneOf(u.Scheme, "https", "tls", "tcp", "udp") {
		return false
	}
	if _, err := netip.ParseAddr(u.Hostname()); err != nil {
		return false
	}
	if u.Port() != "" {
		port, err := strconv.Atoi(u.Port())
		if err != nil || port < 1 || port > 65535 {
			return false
		}
	}
	if u.Scheme != "https" && u.Path != "" {
		return false
	}
	return !strings.ContainsAny(u.Host, "\\%")
}

func (s RunSpec) validateGroups(ids map[string]bool) error {
	groups := make(map[string]Group)
	for _, group := range s.Groups {
		if !validID(group.ID) || ids[group.ID] || !validLabel(group.Name) {
			return errors.New("分组标识或名称无效、重复")
		}
		if err := validateGroup(group); err != nil {
			return err
		}
		ids[group.ID], groups[group.ID] = true, group
	}
	for _, group := range s.Groups {
		seen := make(map[string]bool)
		for _, member := range group.Members {
			if !ids[member] || seen[member] {
				return errors.New("分组包含不存在或重复的成员")
			}
			seen[member] = true
		}
	}
	visiting, depths := make(map[string]bool), make(map[string]int)
	for id := range groups {
		if _, invalid := groupDepth(id, groups, visiting, depths); invalid {
			return errors.New("分组存在循环引用或层级超过限制")
		}
	}
	return nil
}

func validateGroup(group Group) error {
	if !oneOf(group.Type, "select", "urltest") || len(group.Members) == 0 || len(group.Members) > MaxNodes {
		return errors.New("分组类型或成员数量无效")
	}
	if group.TestURL != "" {
		u, err := url.Parse(group.TestURL)
		if err != nil || !oneOf(u.Scheme, "https", "http") || u.Hostname() == "" || u.User != nil || len(group.TestURL) > 2048 {
			return errors.New("分组测速地址无效")
		}
	}
	if group.Interval != "" {
		d, err := time.ParseDuration(group.Interval)
		if err != nil || d < time.Minute || d > 24*time.Hour {
			return errors.New("测速间隔必须在 1 分钟到 24 小时之间")
		}
	}
	return nil
}

func groupDepth(id string, groups map[string]Group, visiting map[string]bool, depths map[string]int) (int, bool) {
	if visiting[id] {
		return 0, true
	}
	if depth, ok := depths[id]; ok {
		return depth, false
	}
	visiting[id] = true
	depth := 1
	for _, member := range groups[id].Members {
		if _, ok := groups[member]; !ok {
			continue
		}
		child, invalid := groupDepth(member, groups, visiting, depths)
		if invalid {
			return 0, true
		}
		depth = max(depth, child+1)
	}
	visiting[id], depths[id] = false, depth
	return depth, depth > 16
}

func ValidID(value string) bool { return idPattern.MatchString(value) }
func validID(value string) bool { return idPattern.MatchString(value) }
func validLabel(value string) bool {
	return strings.TrimSpace(value) != "" && len(value) <= 256 && !strings.ContainsFunc(value, unicode.IsControl)
}
func oneOf(value string, values ...string) bool {
	for _, candidate := range values {
		if value == candidate {
			return true
		}
	}
	return false
}
