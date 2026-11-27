package policy

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestOutboundWhitelist(t *testing.T) {
	valid := []string{
		`{"type":"socks","server":"127.0.0.1","server_port":1080,"version":"5","username":"u","password":"p"}`,
		`{"type":"http","server":"proxy.example","server_port":443,"tls":{"enabled":true,"server_name":"proxy.example"}}`,
		`{"type":"shadowsocks","server":"127.0.0.1","server_port":8388,"method":"aes-128-gcm","password":"p"}`,
		`{"type":"trojan","server":"proxy.example","server_port":443,"password":"p","tls":{"enabled":true}}`,
		`{"type":"vless","server":"proxy.example","server_port":443,"uuid":"e271749c-1f08-450d-8a3a-53066a8b9587","transport":{"type":"ws","path":"/proxy","headers":{"Host":"cdn.example"}}}`,
	}
	for _, raw := range valid {
		got, err := SanitizeOutbound(json.RawMessage(raw))
		if err != nil || !json.Valid(got) {
			t.Fatalf("合法节点被拒绝: %s: %v", raw, err)
		}
	}
}

func TestDangerousAndUnknownFieldsRejected(t *testing.T) {
	fields := strings.Fields("path certificate_path key_path client_certificate_path client_key_path data_directory wrapper_path usages_path static_key_path crl_path credential_path config_path cache_file taildrop_directory state_directory secret_path protect_path private_key_path pid_file mesh_psk_file mca_key_path mca_certificate_path executable_path directory dhcp_lease_files certificate_directory_path certificate_authority_path cache_path plugin plugin_opts detour bind_interface future_field")
	for _, field := range fields {
		raw := `{"type":"socks","server":"127.0.0.1","server_port":1080,"` + field + `":"SECRET"}`
		if _, err := SanitizeOutbound([]byte(raw)); err == nil || strings.Contains(err.Error(), "SECRET") {
			t.Errorf("未拒绝/泄漏 %s: %v", field, err)
		}
	}
	bad := []string{`null`, `[]`, `{} {}`, `{"type":"naive"}`, `{"type":"socks","type":"http"}`, `{"type":"socks","server":"\\\\SECRET\\share","server_port":1080}`, `{"type":"socks","server":"127.0.0.1","server_port":0}`, `{"type":"http","server":"x","server_port":443,"tls":{"enabled":true,"certificate_path":"SECRET"}}`, `{"type":"vless","server":"x","server_port":443,"uuid":"bad"}`}
	for _, raw := range bad {
		if _, err := SanitizeOutbound([]byte(raw)); err == nil || strings.Contains(err.Error(), "SECRET") {
			t.Errorf("应拒绝: %s: %v", raw, err)
		}
	}
}

func TestJSONBoundary(t *testing.T) {
	for _, data := range []string{`null`, `[]`, `{} {}`, `{"x":null}`, `{"x":{"y":1,"y":2}}`, strings.Repeat(`{"x":`, 20) + `0` + strings.Repeat(`}`, 20)} {
		if err := CheckJSON([]byte(data), 16); err == nil {
			t.Errorf("接受不安全JSON: %s", data)
		}
	}
	if err := CheckJSON([]byte(`{"a":[1,2],"b":true}`), 16); err != nil {
		t.Fatal(err)
	}
}
