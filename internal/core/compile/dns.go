package compile

import (
	"ferry/internal/spec"
	"net/url"
	"strconv"
)

func compileDNS(s spec.RunSpec) map[string]any {
	servers := []any{
		dnsServer(s.DNS.Remote, "__dns_remote", defaultProxy(s)),
		dnsServer(s.DNS.Direct, "__dns_direct", "__direct"),
	}
	rules := []any{map[string]any{"clash_mode": "direct", "action": "route", "server": "__dns_direct"}}
	if s.DNS.Mode == "fakeip" {
		servers = append(servers, map[string]any{"type": "fakeip", "tag": "__dns_fakeip", "inet4_range": "198.18.0.0/15", "inet6_range": "fc00::/18"})
		inbounds := []string{"__mixed"}
		if s.Inbound.Tun.Enabled {
			inbounds = append(inbounds, "__tun")
		}
		rules = append(rules, map[string]any{"inbound": inbounds, "query_type": []string{"A", "AAAA"}, "action": "route", "server": "__dns_fakeip"})
	}
	return map[string]any{"servers": servers, "rules": rules, "final": "__dns_remote", "strategy": s.DNS.Strategy}
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
