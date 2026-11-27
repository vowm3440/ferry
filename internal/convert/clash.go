package convert

import (
	"strings"
	"unicode"
)

func clashDraft(raw map[string]any) (draft, string) {
	kind := clashKind(raw["type"])
	known := strings.Fields("name type server port uuid username password flow cipher alterId tls skip-cert-verify servername sni client-fingerprint reality-opts network ws-opts grpc-opts udp obfs obfs-password up down congestion-controller udp-relay-mode ip ipv6 private-key public-key pre-shared-key reserved mtu")
	for key := range raw {
		if !containsKey(known, key) {
			return draft{}, "节点含当前不支持的 Clash 字段"
		}
	}
	o := map[string]any{"type": kind, "server": raw["server"], "server_port": raw["port"]}
	for _, key := range []string{"uuid", "username", "password"} {
		if value, exists := raw[key]; exists {
			o[key] = value
		}
	}
	if kind == "socks" {
		o["version"] = "5"
	}
	if value, exists := raw["cipher"]; exists {
		key := "method"
		if kind == "vmess" {
			key = "security"
		}
		o[key] = value
	}
	if value, exists := raw["alterId"]; exists {
		o["alter_id"] = value
	}
	if udp, exists := raw["udp"]; exists {
		enabled, ok := udp.(bool)
		if !ok {
			return draft{}, "Clash UDP 参数无效"
		}
		if !enabled {
			o["network"] = "tcp"
		}
	}
	if reason := clashTLS(o, raw, kind); reason != "" {
		return draft{}, reason
	}
	if reason := clashTransport(o, raw); reason != "" {
		return draft{}, reason
	}
	if reason := clashExtras(o, raw, kind); reason != "" {
		return draft{}, reason
	}
	name, _ := raw["name"].(string)
	return draft{name: name, outbound: o}, ""
}

func clashKind(value any) string {
	kind, _ := value.(string)
	if kind == "ss" {
		return "shadowsocks"
	}
	if kind == "socks5" {
		return "socks"
	}
	return kind
}

func clashExtras(o, raw map[string]any, kind string) string {
	for key, required := range map[string]string{
		"flow": "vless",
		"obfs": "hysteria2", "obfs-password": "hysteria2", "up": "hysteria2", "down": "hysteria2",
		"congestion-controller": "tuic", "udp-relay-mode": "tuic",
		"ip": "wireguard", "ipv6": "wireguard", "private-key": "wireguard", "public-key": "wireguard",
		"pre-shared-key": "wireguard", "reserved": "wireguard", "mtu": "wireguard",
	} {
		if _, exists := raw[key]; exists && kind != required {
			return "节点含不适用于此协议的参数"
		}
	}
	if kind == "vless" {
		if flow, ok := raw["flow"]; ok {
			o["flow"] = flow
		}
	}
	if kind == "wireguard" {
		return clashWireGuard(o, raw)
	}
	if kind == "hysteria2" {
		if obfs, ok := raw["obfs"]; ok {
			o["obfs"] = map[string]any{"type": obfs, "password": raw["obfs-password"]}
		}
		for key, field := range map[string]string{"up": "up_mbps", "down": "down_mbps"} {
			if value, ok := raw[key]; ok {
				o[field] = value
			}
		}
	}
	if kind == "tuic" {
		for key, field := range map[string]string{"congestion-controller": "congestion_control", "udp-relay-mode": "udp_relay_mode"} {
			if value, ok := raw[key]; ok {
				o[field] = value
			}
		}
	}
	return ""
}

func clashTLS(o, raw map[string]any, kind string) string {
	enabled := kind == "trojan" || kind == "hysteria2" || kind == "tuic" || kind == "anytls"
	if value, exists := raw["tls"]; exists {
		flag, ok := value.(bool)
		if !ok {
			return "Clash TLS 参数无效"
		}
		enabled = flag
	}
	tls := map[string]any{"enabled": enabled}
	if reason := clashTLSFields(tls, raw); reason != "" {
		return reason
	}
	if value, exists := raw["client-fingerprint"]; exists {
		tls["utls"] = map[string]any{"enabled": true, "fingerprint": value}
	}
	if value, exists := raw["reality-opts"]; exists {
		if reason := clashReality(tls, value); reason != "" {
			return reason
		}
	}
	if !enabled && len(tls) > 1 {
		return "TLS 未启用但提供了 TLS 参数"
	}
	if enabled {
		o["tls"] = tls
	}
	return ""
}

func clashReality(tls map[string]any, raw any) string {
	reality, ok := raw.(map[string]any)
	if !ok {
		return "Clash Reality 参数无效"
	}
	for key, val := range reality {
		switch key {
		case "public-key", "short-id":
		case "spider-x":
			spx, ok := val.(string)
			if !ok || len(spx) > 2048 || strings.ContainsFunc(spx, unicode.IsControl) {
				return "Clash Reality spider-x 参数无效"
			}
		default:
			return "Clash Reality 含不支持的字段"
		}
	}
	mapped := map[string]any{"enabled": true, "public_key": reality["public-key"]}
	if pk, ok := reality["public-key"].(string); ok {
		mapped["public_key"] = normalizeRealityKey(pk)
	}
	if id, ok := reality["short-id"]; ok {
		if text, isStr := id.(string); isStr {
			id = normalizeRealityShortID(text)
		}
		mapped["short_id"] = id
	}
	tls["reality"] = mapped
	if tls["utls"] == nil {
		tls["utls"] = map[string]any{"enabled": true, "fingerprint": "chrome"}
	}
	return ""
}

func clashTLSFields(tls, raw map[string]any) string {
	for key, field := range map[string]string{"servername": "server_name", "sni": "server_name", "skip-cert-verify": "insecure"} {
		value, exists := raw[key]
		if !exists {
			continue
		}
		if field == "insecure" {
			if _, ok := value.(bool); !ok {
				return "Clash 证书校验参数无效"
			}
		} else {
			if _, ok := value.(string); !ok {
				return "Clash TLS 服务器名称无效"
			}
		}
		if old, ok := tls[field]; ok && old != value {
			return "Clash TLS 参数冲突"
		}
		tls[field] = value
	}
	return ""
}

func clashTransport(o, raw map[string]any) string {
	network, _ := raw["network"].(string)
	if value, exists := raw["network"]; exists {
		if _, ok := value.(string); !ok {
			return "传输类型无效"
		}
	}
	if network == "ws" && raw["grpc-opts"] != nil || network == "grpc" && raw["ws-opts"] != nil {
		return "传输类型与参数不匹配"
	}
	switch network {
	case "", "tcp":
		if raw["ws-opts"] != nil || raw["grpc-opts"] != nil {
			return "传输类型与参数不匹配"
		}
	case "ws":
		transport := map[string]any{"type": "ws", "path": "/"}
		if value, exists := raw["ws-opts"]; exists {
			options, ok := value.(map[string]any)
			if !ok {
				return "WebSocket 参数无效"
			}
			for key, value := range options {
				if key != "path" && key != "headers" {
					return "WebSocket 含不支持的字段"
				}
				transport[key] = value
			}
		}
		o["transport"] = transport
	case "grpc":
		transport := map[string]any{"type": "grpc"}
		if value, exists := raw["grpc-opts"]; exists {
			options, ok := value.(map[string]any)
			if !ok {
				return "gRPC 参数无效"
			}
			for key, value := range options {
				if key != "grpc-service-name" {
					return "gRPC 含不支持的字段"
				}
				transport["service_name"] = value
			}
		}
		o["transport"] = transport
	default:
		return "当前不支持此 Clash 传输类型"
	}
	return ""
}
