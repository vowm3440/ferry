package policy

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

const boundaryUUID = "11111111-2222-4333-8444-555555555555"
const boundaryPublicKey = "AQIDBAUGBwgJCgsMDQ4PEBESExQVFhcYGRobHB0eHyA"

func assertOutboundPreserved(t *testing.T, raw string) {
	t.Helper()
	clean, err := SanitizeOutbound([]byte(raw))
	if err != nil {
		t.Fatalf("supported outbound rejected: %v", err)
	}
	var want, got any
	if err := json.Unmarshal([]byte(raw), &want); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(clean, &got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatal("sanitization changed or dropped a supported protocol field")
	}
}

func assertOutboundRejected(t *testing.T, raw string) {
	t.Helper()
	clean, err := SanitizeOutbound([]byte(raw))
	if err == nil || len(clean) != 0 {
		t.Fatal("unsafe or unsupported outbound returned usable configuration")
	}
	if strings.Contains(err.Error(), "PRIVATE-SENTINEL") {
		t.Fatal("validation error exposed input credentials or path")
	}
}

func outboundWithFields(kind, fields string) string {
	return `{"type":"` + kind + `","server":"edge.example.com","server_port":443,` + fields + `}`
}

func TestModernProtocolsPreserveSupportedOptions(t *testing.T) {
	for _, tc := range []struct{ name, kind, fields string }{
		{"vmess_plain", "vmess", `"uuid":"` + boundaryUUID + `","security":"zero","alter_id":0,"network":"udp"`},
		{"vmess_grpc", "vmess", `"uuid":"` + boundaryUUID + `","security":"aes-128-gcm","alter_id":16,"network":"tcp","tls":{"enabled":true,"server_name":"cdn.example.com","alpn":["h2"],"utls":{"enabled":true,"fingerprint":"firefox"}},"transport":{"type":"grpc","service_name":"proxy.Service"}`},
		{"hy2_obfs", "hysteria2", `"password":"PRIVATE-SENTINEL","tls":{"enabled":true,"server_name":"edge.example.com","insecure":false,"alpn":["h3"]},"up_mbps":125,"down_mbps":0,"obfs":{"type":"salamander","password":"separate-obfs-secret"},"network":"udp"`},
		{"tuic_early_data", "tuic", `"uuid":"` + boundaryUUID + `","password":"PRIVATE-SENTINEL","tls":{"enabled":true,"disable_sni":true,"insecure":true,"alpn":["h3"]},"congestion_control":"bbr","udp_relay_mode":"quic","zero_rtt_handshake":true,"network":"tcp"`},
		{"tuic_native", "tuic", `"uuid":"` + boundaryUUID + `","password":"PRIVATE-SENTINEL","tls":{"enabled":true},"congestion_control":"new_reno","udp_relay_mode":"native","zero_rtt_handshake":false`},
		{"anytls", "anytls", `"password":"PRIVATE-SENTINEL","tls":{"enabled":true,"server_name":"cdn.example.com","utls":{"enabled":true,"fingerprint":"chrome"}}`},
	} {
		t.Run(tc.name, func(t *testing.T) { assertOutboundPreserved(t, outboundWithFields(tc.kind, tc.fields)) })
	}
}

func TestModernProtocolsRejectMissingCredentialsAndTLS(t *testing.T) {
	for _, kind := range []string{"vmess", "tuic", "hysteria2", "anytls"} {
		valid := map[string]any{"password": "PRIVATE-SENTINEL", "tls": map[string]any{"enabled": true}}
		credential := "password"
		if kind == "vmess" || kind == "tuic" {
			valid["uuid"] = boundaryUUID
		}
		if kind == "vmess" {
			delete(valid, "password")
			credential = "uuid"
		}
		for _, field := range []string{credential, "tls"} {
			if kind == "vmess" && field == "tls" {
				continue // VMess supports an explicitly plain transport.
			}
			for _, invalid := range []any{nil, "", false, 123} {
				t.Run(kind+"/"+field+"/"+testValueName(invalid), func(t *testing.T) {
					assertOutboundRejected(t, mutatedOutbound(t, kind, valid, field, invalid))
				})
			}
		}
		if kind != "vmess" {
			t.Run(kind+"/disabled_tls", func(t *testing.T) {
				assertOutboundRejected(t, mutatedOutbound(t, kind, valid, "tls", map[string]any{"enabled": false}))
			})
		}
	}
}

func testValueName(value any) string {
	if value == nil {
		return "missing"
	}
	encoded, _ := json.Marshal(value)
	return string(encoded)
}

// Each call starts from a valid fixture and changes exactly one public field.
func mutatedOutbound(t *testing.T, kind string, fields map[string]any, key string, value any) string {
	t.Helper()
	outbound := map[string]any{"type": kind, "server": "edge.example.com", "server_port": 443}
	for field, entry := range fields {
		outbound[field] = entry
	}
	if value == nil {
		delete(outbound, key)
	} else {
		outbound[key] = value
	}
	data, err := json.Marshal(outbound)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestVMessRejectsUnsupportedSecurityAndAlterID(t *testing.T) {
	valid := map[string]any{"uuid": boundaryUUID}
	for _, tc := range []struct {
		field string
		value any
	}{
		{"uuid", "not-a-uuid"}, {"security", "aes-256-gcm"}, {"security", 1},
		{"alter_id", -1}, {"alter_id", 0.5}, {"alter_id", 65536}, {"alter_id", "0"},
		{"network", "icmp"}, {"network", []string{"tcp", "udp"}},
	} {
		t.Run(tc.field+"/"+testValueName(tc.value), func(t *testing.T) {
			assertOutboundRejected(t, mutatedOutbound(t, "vmess", valid, tc.field, tc.value))
		})
	}
}

func TestHysteriaRejectsInvalidBandwidthAndObfuscation(t *testing.T) {
	valid := map[string]any{"password": "PRIVATE-SENTINEL", "tls": map[string]any{"enabled": true}}
	for _, field := range []string{"up_mbps", "down_mbps"} {
		for _, value := range []any{-1, 0.1, "100", 1000001} {
			t.Run(field+"/"+testValueName(value), func(t *testing.T) {
				assertOutboundRejected(t, mutatedOutbound(t, "hysteria2", valid, field, value))
			})
		}
	}
	for name, obfs := range map[string]any{
		"scalar": "salamander", "missing_type": map[string]any{"password": "p"},
		"unsupported_type":    map[string]any{"type": "gecko", "password": "p"},
		"missing_password":    map[string]any{"type": "salamander"},
		"empty_password":      map[string]any{"type": "salamander", "password": ""},
		"control_in_password": map[string]any{"type": "salamander", "password": "p\nsecret"},
		"oversized_password":  map[string]any{"type": "salamander", "password": strings.Repeat("p", 8193)},
		"unsafe_nested_path":  map[string]any{"type": "salamander", "password": "p", "secret_path": `C:\PRIVATE-SENTINEL`},
	} {
		t.Run(name, func(t *testing.T) { assertOutboundRejected(t, mutatedOutbound(t, "hysteria2", valid, "obfs", obfs)) })
	}
}

func TestTUICRejectsInvalidIdentityAndTransportOptions(t *testing.T) {
	valid := map[string]any{"uuid": boundaryUUID, "password": "PRIVATE-SENTINEL", "tls": map[string]any{"enabled": true}}
	for _, tc := range []struct {
		field string
		value any
	}{
		{"uuid", nil}, {"uuid", ""}, {"uuid", "not-a-uuid"},
		{"congestion_control", "reno"}, {"udp_relay_mode", "stream"},
		{"zero_rtt_handshake", "true"}, {"zero_rtt_handshake", 1},
		{"transport", map[string]any{"type": "grpc"}},
	} {
		t.Run(tc.field+"/"+testValueName(tc.value), func(t *testing.T) {
			assertOutboundRejected(t, mutatedOutbound(t, "tuic", valid, tc.field, tc.value))
		})
	}
}
