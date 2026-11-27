package convert

import (
	"encoding/base64"
	"net/url"
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

func TestV2rayNVLESSRealityShareLinksAndClash(t *testing.T) {
	rawKey := make([]byte, 32)
	for i := range rawKey {
		rawKey[i] = byte(i + 0x80)
	}
	urlKey := base64.RawURLEncoding.EncodeToString(rawKey)
	stdKey := url.QueryEscape(base64.StdEncoding.EncodeToString(rawKey))

	t.Run("v2rayn_vision_tcp_spx", func(t *testing.T) {
		link := "vless://" + testUUID + "@85.149.216.35:443?encryption=none&flow=xtls-rprx-vision&security=reality&sni=nlnet.nl&fp=chrome&pbk=" + urlKey + "&sid=d9da6d58ff569e4f&spx=%2F&type=tcp&headerType=none#jp-vless"
		node, o := one(t, link)
		if node.Tag != "jp-vless" || o["flow"] != "xtls-rprx-vision" || o["server"] != "85.149.216.35" {
			t.Fatalf("unexpected node/outbound: %+v %v", node, o)
		}
		reality := o["tls"].(map[string]any)["reality"].(map[string]any)
		if reality["public_key"] != urlKey || reality["short_id"] != "d9da6d58ff569e4f" {
			t.Fatalf("reality = %v", reality)
		}
	})

	t.Run("v2rayn_raw_padded_pbk_odd_sid", func(t *testing.T) {
		link := "vless://" + testUUID + "@edge.example.com:443?encryption=none&security=reality&sni=nlnet.nl&pbk=" + stdKey + "&sid=abc&spx=%2F&type=raw#raw-node"
		_, o := one(t, link)
		reality := o["tls"].(map[string]any)["reality"].(map[string]any)
		if reality["public_key"] != urlKey || reality["short_id"] != "abc0" {
			t.Fatalf("normalized reality = %v", reality)
		}
	})

	t.Run("v2rayn_grpc_mode_gun", func(t *testing.T) {
		link := "vless://" + testUUID + "@edge.example.com:443?encryption=none&security=reality&sni=nlnet.nl&fp=firefox&pbk=" + urlKey + "&sid=0102&spx=%2Fapi&type=grpc&serviceName=gun-svc&mode=gun#grpc-node"
		_, o := one(t, link)
		if o["transport"].(map[string]any)["service_name"] != "gun-svc" {
			t.Fatalf("transport = %v", o["transport"])
		}
	})

	t.Run("clash_vless_reality_vision_spiderx", func(t *testing.T) {
		doc := "proxies:\n  - name: jp-vless\n    type: vless\n    server: 85.149.216.35\n    port: 443\n    uuid: " + testUUID + "\n    tls: true\n    flow: xtls-rprx-vision\n    servername: nlnet.nl\n    reality-opts:\n      public-key: " + urlKey + "\n      short-id: d9da6d58ff569e4f\n      spider-x: /\n"
		_, o := one(t, doc)
		tls := o["tls"].(map[string]any)
		if o["flow"] != "xtls-rprx-vision" || tls["utls"].(map[string]any)["fingerprint"] != "chrome" {
			t.Fatalf("clash reality outbound = %v", o)
		}
	})
}
