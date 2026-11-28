package compile

import (
	"ferry/internal/routing"
	"ferry/internal/spec"
	"net/url"
	"strconv"
)

func compileDNS(s spec.RunSpec) map[string]any {
	servers := []any{
		dnsServer(s.DNS.Remote, "__dns_remote", defaultProxy(s)),
		dnsServer(s.DNS.Direct, "__dns_direct", "__direct"),
	}
	for _, srv := range s.DNS.Servers {
		detour := "__direct"
		if srv.Detour == "proxy" {
			detour = defaultProxy(s)
		}
		servers = append(servers, dnsServer(srv.Address, customDNSTag(srv.ID), detour))
	}
	rules := []any{map[string]any{"clash_mode": "direct", "action": "route", "server": "__dns_direct"}}
	for _, rule := range s.DNS.Rules {
		if !rule.Enabled {
			continue
		}
		rules = append(rules, compileDNSRule(s, rule))
	}
	if s.DNS.Mode == "fakeip" {
		fakeip := map[string]any{"type": "fakeip", "tag": "__dns_fakeip", "inet4_range": "198.18.0.0/15"}
		qtypes := []string{"A"}
		if s.DNS.Strategy != "ipv4_only" {
			fakeip["inet6_range"] = "fc00::/18"
			qtypes = append(qtypes, "AAAA")
		}
		servers = append(servers, fakeip)
		inbounds := []string{"__mixed"}
		if s.Inbound.Tun.Enabled {
			inbounds = append(inbounds, "__tun")
		}
		rules = append(rules, map[string]any{"inbound": inbounds, "query_type": qtypes, "action": "route", "server": "__dns_fakeip"})
	}
	return map[string]any{"servers": servers, "rules": rules, "final": "__dns_remote", "strategy": s.DNS.Strategy}
}

func customDNSTag(id string) string { return "__dns_custom_" + id }

func compileDNSRule(s spec.RunSpec, rule spec.DNSRule) map[string]any {
	compiled := routing.BuildDNSDomains(s, rule.Domains)
	compiled["action"] = "route"
	switch rule.Server {
	case "direct":
		compiled["server"] = "__dns_direct"
	case "remote":
		compiled["server"] = "__dns_remote"
	default:
		compiled["server"] = customDNSTag(rule.Server)
	}
	return compiled
}

// The URLs have already passed spec validation, including a literal IP host.
func dnsServer(address, tag, detour string) map[string]any {
	u, _ := url.Parse(address)
	result := map[string]any{"type": u.Scheme, "tag": tag, "server": u.Hostname()}
	if detour != "__direct" {
		result["detour"] = detour
	}
	if u.Port() != "" {
		port, _ := strconv.Atoi(u.Port())
		result["server_port"] = port
	}
	if u.Scheme == "https" {
		path := u.EscapedPath()
		if path == "" {
			path = "/dns-query"
		}
		result["path"] = path
	}
	return result
}
