package policy

import (
	"strings"
	"testing"
)

func vlessWithTLS(fields string) string {
	return outboundWithFields("vless", `"uuid":"`+boundaryUUID+`","tls":{`+fields+`}`)
}

func TestTLSRejectsOptionsWhenDisabled(t *testing.T) {
	for name, field := range map[string]string{
		"utls":        `"utls":{"enabled":true,"fingerprint":"chrome"}`,
		"reality":     `"reality":{"enabled":true,"public_key":"` + boundaryPublicKey + `"}`,
		"server_name": `"server_name":"cdn.example.com"`,
		"insecure":    `"insecure":true`, "alpn": `"alpn":["h2"]`, "disable_sni": `"disable_sni":true`,
	} {
		t.Run(name+"/false", func(t *testing.T) { assertOutboundRejected(t, vlessWithTLS(`"enabled":false,`+field)) })
		t.Run(name+"/missing", func(t *testing.T) { assertOutboundRejected(t, vlessWithTLS(field)) })
	}
}

func TestTLSPreservesRealityAndOptionalShortID(t *testing.T) {
	for _, shortID := range []string{"", `,"short_id":""`, `,"short_id":"01aB89cd"`} {
		raw := `"enabled":true,"server_name":"cdn.example.com","utls":{"enabled":true,"fingerprint":"chrome"},"reality":{"enabled":true,"public_key":"` + boundaryPublicKey + `"` + shortID + `}`
		assertOutboundPreserved(t, vlessWithTLS(raw))
	}
	assertOutboundPreserved(t, vlessWithTLS(`"enabled":false`))
}

func TestTLSRejectsInvalidRealityKeysAndShortIDs(t *testing.T) {
	for name, reality := range map[string]string{
		"not_object": `true`, "missing_enabled": `{"public_key":"` + boundaryPublicKey + `"}`,
		"disabled_reality": `{"enabled":false,"public_key":"` + boundaryPublicKey + `"}`,
		"missing_key":      `{"enabled":true}`, "wrong_key_type": `{"enabled":true,"public_key":32}`,
		"malformed_key":       `{"enabled":true,"public_key":"not/base64!"}`,
		"short_key":           `{"enabled":true,"public_key":"AQID"}`,
		"long_key":            `{"enabled":true,"public_key":"` + boundaryPublicKey + `AAAA"}`,
		"wrong_short_id_type": `{"enabled":true,"public_key":"` + boundaryPublicKey + `","short_id":12}`,
		"odd_short_id":        `{"enabled":true,"public_key":"` + boundaryPublicKey + `","short_id":"123"}`,
		"nonhex_short_id":     `{"enabled":true,"public_key":"` + boundaryPublicKey + `","short_id":"zz"}`,
		"long_short_id":       `{"enabled":true,"public_key":"` + boundaryPublicKey + `","short_id":"0123456789abcdef00"}`,
		"private_key_path":    `{"enabled":true,"public_key":"` + boundaryPublicKey + `","private_key_path":"PRIVATE-SENTINEL"}`,
	} {
		t.Run(name, func(t *testing.T) { assertOutboundRejected(t, vlessWithTLS(`"enabled":true,"reality":`+reality)) })
	}
}

func TestTLSRejectsMalformedFieldsAndNestedUTLSOptions(t *testing.T) {
	for name, field := range map[string]string{
		"boolean_as_string": `"enabled":"true"`, "invalid_insecure": `"enabled":true,"insecure":1`,
		"invalid_disable_sni": `"enabled":true,"disable_sni":"false"`,
		"server_name_type":    `"enabled":true,"server_name":42`, "server_name_url": `"enabled":true,"server_name":"https://cdn.example.com"`,
		"alpn_scalar": `"enabled":true,"alpn":"h2"`, "alpn_number": `"enabled":true,"alpn":[2]`,
		"alpn_empty_item": `"enabled":true,"alpn":[""]`, "alpn_oversized_item": `"enabled":true,"alpn":["` + strings.Repeat("a", 256) + `"]`,
		"alpn_too_many": `"enabled":true,"alpn":[` + strings.Repeat(`"h2",`, 16) + `"h3"]`,
		"utls_scalar":   `"enabled":true,"utls":"chrome"`, "utls_missing_flag": `"enabled":true,"utls":{"fingerprint":"chrome"}`,
		"utls_disabled":            `"enabled":true,"utls":{"enabled":false,"fingerprint":"chrome"}`,
		"utls_missing_fingerprint": `"enabled":true,"utls":{"enabled":true}`,
		"utls_unknown_fingerprint": `"enabled":true,"utls":{"enabled":true,"fingerprint":"future-browser"}`,
		"utls_path":                `"enabled":true,"utls":{"enabled":true,"fingerprint":"chrome","config_path":"PRIVATE-SENTINEL"}`,
		"tls_certificate_path":     `"enabled":true,"certificate_path":"PRIVATE-SENTINEL"`,
	} {
		t.Run(name, func(t *testing.T) { assertOutboundRejected(t, vlessWithTLS(field)) })
	}
}

func TestTransportsPreserveURLPathAndHeaderValues(t *testing.T) {
	for _, transport := range []string{
		`{"type":"ws","path":"/proxy?token=PRIVATE-SENTINEL","headers":{"Host":"cdn.example.com","X-Session":"opaque-token"}}`,
		`{"type":"http","path":"/proxy","headers":{"User-Agent":"Ferry test"}}`,
		`{"type":"grpc","service_name":"proxy.Service/With-Separator"}`,
		`{"type":"grpc"}`,
	} {
		assertOutboundPreserved(t, outboundWithFields("vless", `"uuid":"`+boundaryUUID+`","transport":`+transport))
	}
}

func TestTransportsRejectUnknownOrFilesystemOptions(t *testing.T) {
	for name, transport := range map[string]string{
		"scalar": `"ws"`, "unknown_type": `{"type":"quic"}`, "missing_type": `{"path":"/proxy"}`,
		"grpc_filesystem_path": `{"type":"grpc","path":"C:\\PRIVATE-SENTINEL"}`,
		"grpc_control":         `{"type":"grpc","service_name":"test\nPRIVATE-SENTINEL"}`,
		"grpc_long_service":    `{"type":"grpc","service_name":"` + strings.Repeat("s", 2049) + `"}`,
		"grpc_service_type":    `{"type":"grpc","service_name":1}`,
		"ws_relative_path":     `{"type":"ws","path":"relative"}`, "ws_unc_path": `{"type":"ws","path":"//PRIVATE-SENTINEL/share"}`,
		"ws_backslash_path": `{"type":"ws","path":"/PRIVATE-SENTINEL\\share"}`,
		"ws_long_path":      `{"type":"ws","path":"/` + strings.Repeat("p", 2048) + `"}`,
		"ws_path_type":      `{"type":"ws","path":1}`, "ws_config_path": `{"type":"ws","config_path":"PRIVATE-SENTINEL"}`,
		"headers_not_object": `{"type":"http","headers":[]}`, "empty_header": `{"type":"ws","headers":{"":"value"}}`,
		"header_name_injection":  `{"type":"ws","headers":{"X-Header\r\nInjected":"value"}}`,
		"header_value_injection": `{"type":"ws","headers":{"X-Header":"value\r\nInjected: PRIVATE-SENTINEL"}}`,
		"header_value_type":      `{"type":"ws","headers":{"X-Header":["a","b"]}}`,
		"header_name_too_long":   `{"type":"ws","headers":{"` + strings.Repeat("X", 129) + `":"value"}}`,
	} {
		t.Run(name, func(t *testing.T) {
			assertOutboundRejected(t, outboundWithFields("vless", `"uuid":"`+boundaryUUID+`","transport":`+transport))
		})
	}
}
