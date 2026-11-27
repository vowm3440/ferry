package convert

import (
	"net"
	"net/url"
	"strconv"
	"strings"
)

func parseModernURL(line, kind string) (draft, string) {
	if kind == "hy2" {
		kind = "hysteria2"
	}
	if u, err := url.Parse(line); err == nil && u.Hostname() != "" && u.Port() == "" {
		u.Host = net.JoinHostPort(u.Hostname(), "443")
		line = u.String()
	}
	u, q, reason := parseURL(line)
	if reason != "" {
		return draft{}, reason
	}
	if security := q.Get("security"); security != "" && security != "tls" {
		return draft{}, "此协议必须使用 TLS"
	}
	if reason = validateTLSParams(q, true); reason != "" {
		return draft{}, reason
	}
	o := base(kind, u)
	password := u.User.Username()
	if kind == "tuic" {
		o["uuid"] = password
		password, _ = u.User.Password()
	} else if suffix, present := u.User.Password(); present {
		password += ":" + suffix
	}
	if password == "" {
		return draft{}, "缺少密码"
	}
	o["password"], o["tls"] = password, tlsOptions(q)
	known := []string{"security", "sni", "peer", "alpn", "insecure", "allowInsecure"}
	switch kind {
	case "hysteria2":
		known = append(known, "obfs", "obfs-password", "upmbps", "downmbps")
		reason = applyHysteria(o, q)
	case "tuic":
		known = append(known, "congestion_control", "udp_relay_mode", "zero_rtt_handshake", "disable_sni")
		reason = applyTUIC(o, q)
	case "anytls":
		known = append(known, "fp")
	}
	if reason != "" {
		return draft{}, reason
	}
	return draft{name: u.Fragment, outbound: o}, unsupportedParam(q, known...)
}

func applyHysteria(o map[string]any, q url.Values) string {
	if q.Get("obfs") != "" {
		if q.Get("obfs") != "salamander" {
			return "暂不支持此混淆类型"
		}
		o["obfs"] = map[string]any{"type": "salamander", "password": q.Get("obfs-password")}
	} else if q.Get("obfs-password") != "" {
		return "未指定混淆类型"
	}
	for key, field := range map[string]string{"upmbps": "up_mbps", "downmbps": "down_mbps"} {
		if raw := q.Get(key); raw != "" {
			value, err := strconv.Atoi(raw)
			if err != nil {
				return "带宽参数无效"
			}
			o[field] = value
		}
	}
	return ""
}

func applyTUIC(o map[string]any, q url.Values) string {
	for _, key := range []string{"congestion_control", "udp_relay_mode"} {
		if value := q.Get(key); value != "" {
			o[key] = value
		}
	}
	for _, key := range []string{"zero_rtt_handshake", "disable_sni"} {
		value := q.Get(key)
		if value == "" {
			continue
		}
		switch strings.ToLower(value) {
		case "0", "1", "true", "false":
		default:
			return "TUIC 布尔参数无效"
		}
		if key == "disable_sni" {
			o["tls"].(map[string]any)[key] = truthy(value)
		} else {
			o[key] = truthy(value)
		}
	}
	return ""
}
