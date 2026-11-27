package convert

import (
	"encoding/base64"
	"testing"
)

func TestModernURIProtocols(t *testing.T) {
	vmess := `{"v":"2","ps":"VMess","add":"edge.example.com","port":"443","id":"` + testUUID + `","aid":"0","scy":"auto","net":"ws","type":"none","host":"cdn.example.com","path":"/ws","tls":"tls","sni":"edge.example.com"}`
	for _, tc := range []struct{ link, kind string }{
		{"vmess://" + base64.StdEncoding.EncodeToString([]byte(vmess)), "vmess"},
		{"hysteria2://test-password@edge.example.com:443?sni=edge.example.com&obfs=salamander&obfs-password=test-obfs", "hysteria2"},
		{"tuic://" + testUUID + ":test-password@edge.example.com:443?congestion_control=bbr&udp_relay_mode=native&alpn=h3", "tuic"},
		{"anytls://test-password@edge.example.com:443?sni=edge.example.com", "anytls"},
	} {
		t.Run(tc.kind, func(t *testing.T) {
			_, value := one(t, tc.link)
			if value["type"] != tc.kind {
				t.Fatal("wrong protocol")
			}
		})
	}
}

func TestVLESSRealityAndGRPC(t *testing.T) {
	key := base64.RawURLEncoding.EncodeToString(make([]byte, 32))
	_, value := one(t, "vless://"+testUUID+"@edge.example.com:443?security=reality&sni=edge.example.com&pbk="+key+"&sid=abcd&fp=chrome&type=grpc&serviceName=test-service")
	tls := value["tls"].(map[string]any)
	if tls["reality"].(map[string]any)["public_key"] != key || value["transport"].(map[string]any)["service_name"] != "test-service" {
		t.Fatal("Reality/GRPC parameters were lost")
	}
}
