package policy

import (
	"encoding/json"
	"errors"
	"net/netip"
	"regexp"
	"strings"
	"unicode"
)

var errOutbound = errors.New("节点出站配置无效或包含当前阶段不支持的字段")
var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// SanitizeOutbound validates every accepted field and preserves its value.
// Unsupported features are rejected, never silently removed.
func SanitizeOutbound(data json.RawMessage) (json.RawMessage, error) {
	if len(data) > 64<<10 || CheckJSON(data, 12) != nil {
		return nil, errOutbound
	}
	var object map[string]any
	if json.Unmarshal(data, &object) != nil {
		return nil, errOutbound
	}
	if !validOutbound(object) {
		return nil, errOutbound
	}
	result, err := json.Marshal(object)
	if err != nil {
		return nil, errOutbound
	}
	return result, nil
}

func validOutbound(o map[string]any) bool {
	kind, _ := o["type"].(string)
	fields, ok := protocolFields[kind]
	if !ok || !allowed(o, "type server server_port "+fields) {
		return false
	}
	server, ok := o["server"].(string)
	if !ok || !Host(server) || !integer(o["server_port"], 1, 65535) {
		return false
	}
	if !optionalStrings(o, "username password method version uuid flow network security congestion_control udp_relay_mode", 8192) {
		return false
	}
	if network, ok := o["network"]; ok && network != "tcp" && network != "udp" {
		return false
	}
	if tls, ok := o["tls"]; ok && !validTLS(tls) {
		return false
	}
	if transport, ok := o["transport"]; ok && !validTransport(transport) {
		return false
	}
	return validProtocol(kind, o)
}

var protocolFields = map[string]string{
	"socks":       "version username password network",
	"http":        "username password tls",
	"shadowsocks": "method password network",
	"trojan":      "password tls transport network",
	"vless":       "uuid flow tls transport network",
	"vmess":       "uuid security alter_id tls transport network",
	"hysteria2":   "password tls obfs up_mbps down_mbps network",
	"tuic":        "uuid password tls congestion_control udp_relay_mode zero_rtt_handshake network",
	"anytls":      "password tls",
	"wireguard":   "local_address private_key peer_public_key pre_shared_key reserved mtu network",
}

func validProtocol(kind string, o map[string]any) bool {
	switch kind {
	case "socks":
		v, exists := o["version"]
		return !exists || v == "4" || v == "4a" || v == "5"
	case "shadowsocks":
		return requiredString(o, "password") && contains([]string{"aes-128-gcm", "aes-256-gcm", "chacha20-ietf-poly1305"}, o["method"])
	case "trojan":
		tls, _ := o["tls"].(map[string]any)
		return requiredString(o, "password") && tls["enabled"] == true
	case "vless":
		uuid, _ := o["uuid"].(string)
		flow, _ := o["flow"].(string)
		return uuidPattern.MatchString(uuid) && (flow == "" || flow == "xtls-rprx-vision")
	case "vmess", "hysteria2", "tuic", "anytls":
		return validModernProtocol(kind, o)
	case "wireguard":
		return validWireGuard(o)
	default:
		return true
	}
}

// Host accepts only a literal IP or DNS host, with no path, port or scope ID.
func Host(value string) bool {
	if len(value) == 0 || len(value) > 253 {
		return false
	}
	if addr, err := netip.ParseAddr(value); err == nil {
		return addr.Zone() == ""
	}
	for _, label := range strings.Split(strings.TrimSuffix(value, "."), ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, c := range label {
			if c != '-' && !(c >= 'a' && c <= 'z') && !(c >= 'A' && c <= 'Z') && !(c >= '0' && c <= '9') {
				return false
			}
		}
	}
	return true
}

func allowed(o map[string]any, fields string) bool {
	set := strings.Fields(fields)
	for key := range o {
		if !contains(set, key) {
			return false
		}
	}
	return true
}

func contains(values []string, value any) bool {
	for _, item := range values {
		if value == item {
			return true
		}
	}
	return false
}

func integer(value any, min, max float64) bool {
	number, ok := value.(float64)
	return ok && number >= min && number <= max && number == float64(int64(number))
}

func optionalStrings(o map[string]any, keys string, limit int) bool {
	for _, key := range strings.Fields(keys) {
		value, exists := o[key]
		if !exists {
			continue
		}
		text, ok := value.(string)
		if !ok || len(text) > limit || strings.ContainsFunc(text, unicode.IsControl) {
			return false
		}
	}
	return true
}

func requiredString(o map[string]any, key string) bool {
	value, ok := o[key].(string)
	return ok && value != ""
}
