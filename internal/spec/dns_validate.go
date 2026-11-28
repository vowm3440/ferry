package spec

import (
	"errors"
	"strings"
)

func (s RunSpec) validateDNSRouting() error {
	if len(s.DNS.Servers) > MaxDNSServers {
		return errors.New("自定义 DNS 服务器数量超过限制")
	}
	servers := map[string]bool{"direct": true, "remote": true}
	for _, srv := range s.DNS.Servers {
		if err := validateDNSServer(srv, servers); err != nil {
			return err
		}
		servers[srv.ID] = true
	}
	return s.validateDNSRules(servers)
}

func validateDNSServer(srv DNSServer, seen map[string]bool) error {
	if !validID(srv.ID) || seen[srv.ID] || !validLabel(srv.Name) {
		return errors.New("自定义 DNS 服务器标识或名称无效、重复")
	}
	if !validDNS(srv.Address) {
		return errors.New("DNS 仅支持使用 IP 地址的 HTTPS、TLS、TCP 或 UDP 服务器")
	}
	if srv.Detour != "" && !oneOf(srv.Detour, "direct", "proxy") {
		return errors.New("自定义 DNS 出口设置无效")
	}
	return nil
}

func (s RunSpec) validateDNSRules(servers map[string]bool) error {
	if len(s.DNS.Rules) > MaxDNSRules {
		return errors.New("分流 DNS 规则数量超过限制")
	}
	seen := make(map[string]bool, len(s.DNS.Rules))
	for _, r := range s.DNS.Rules {
		if seen[r.ID] {
			return errors.New("分流 DNS 规则标识重复")
		}
		if err := s.validateDNSRule(r, servers); err != nil {
			return err
		}
		seen[r.ID] = true
	}
	return nil
}

func (s RunSpec) validateDNSRule(r DNSRule, servers map[string]bool) error {
	if !validID(r.ID) || !validLabel(r.Name) {
		return errors.New("分流 DNS 规则标识或名称无效")
	}
	if !servers[r.Server] {
		return errors.New("分流 DNS 规则目标服务器不存在")
	}
	if len(r.Domains) == 0 || len(r.Domains) > MaxRuleItems {
		return errors.New("分流 DNS 规则须包含 1 到 256 个域名或规则集")
	}
	for _, item := range r.Domains {
		if err := s.validateDNSDomain(item); err != nil {
			return err
		}
	}
	return nil
}

func (s RunSpec) validateDNSDomain(item string) error {
	if !validRuleText(item, 253) {
		return errors.New("分流 DNS 域名条件无效")
	}
	lower := strings.ToLower(item)
	if strings.HasPrefix(lower, "geoip:") {
		return errors.New("分流 DNS 规则仅支持域名或域名规则集，不支持 geoip")
	}
	if _, ok := s.RuleSetID(item); ok {
		return nil
	}
	if strings.HasPrefix(lower, "geosite:") || strings.HasPrefix(lower, CustomRuleSetPrefix) {
		return errors.New("分流 DNS 引用的规则集不存在")
	}
	if !validRuleDomain(item) {
		return errors.New("分流 DNS 域名格式无效")
	}
	return nil
}
