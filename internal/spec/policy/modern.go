package policy

import (
	"encoding/base64"
	"encoding/hex"
)

func validModernProtocol(kind string, o map[string]any) bool {
	uuid, _ := o["uuid"].(string)
	if kind == "vmess" {
		security, _ := o["security"].(string)
		return uuidPattern.MatchString(uuid) && contains([]string{"", "auto", "none", "zero", "aes-128-cfb", "aes-128-gcm", "chacha20-poly1305"}, security) && optionalInteger(o, "alter_id", 0, 65535)
	}
	tls, _ := o["tls"].(map[string]any)
	if !requiredString(o, "password") || tls["enabled"] != true {
		return false
	}
	if kind == "tuic" {
		if !uuidPattern.MatchString(uuid) || !optionalChoice(o, "congestion_control", "cubic", "new_reno", "bbr") || !optionalChoice(o, "udp_relay_mode", "native", "quic") {
			return false
		}
		if value, exists := o["zero_rtt_handshake"]; exists {
			if _, ok := value.(bool); !ok {
				return false
			}
		}
	}
	if kind == "hysteria2" {
		if !optionalInteger(o, "up_mbps", 0, 1000000) || !optionalInteger(o, "down_mbps", 0, 1000000) {
			return false
		}
		if value, exists := o["obfs"]; exists {
			obfs, ok := value.(map[string]any)
			if !ok || !allowed(obfs, "type password") || obfs["type"] != "salamander" || !requiredString(obfs, "password") || !optionalStrings(obfs, "password", 8192) {
				return false
			}
		}
	}
	return true
}
func optionalInteger(o map[string]any, key string, min, max float64) bool {
	value, exists := o[key]
	return !exists || integer(value, min, max)
}
func optionalChoice(o map[string]any, key string, values ...string) bool {
	value, exists := o[key]
	return !exists || contains(values, value)
}

func validUTLS(value any) bool {
	o, ok := value.(map[string]any)
	return ok && allowed(o, "enabled fingerprint") && o["enabled"] == true && contains([]string{"chrome", "firefox", "edge", "safari", "360", "qq", "ios", "android", "random", "randomized", "chrome_psk", "chrome_psk_shuffle", "chrome_padding_psk_shuffle", "chrome_pq", "chrome_pq_psk"}, o["fingerprint"])
}
func validReality(value any) bool {
	o, ok := value.(map[string]any)
	if !ok || !allowed(o, "enabled public_key short_id") || o["enabled"] != true {
		return false
	}
	key, ok := o["public_key"].(string)
	if !ok {
		return false
	}
	decoded, err := base64.RawURLEncoding.DecodeString(key)
	if err != nil || len(decoded) != 32 {
		return false
	}
	if short, exists := o["short_id"]; exists {
		text, ok := short.(string)
		if !ok || len(text) > 16 {
			return false
		}
		if _, err := hex.DecodeString(text); err != nil {
			return false
		}
	}
	return true
}
