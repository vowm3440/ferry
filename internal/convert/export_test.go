package convert

import (
	"encoding/base64"
	"testing"
)

func TestExportRoundTripsSupportedLinks(t *testing.T) {
	vmess := `{"v":"2","ps":"vmess","add":"edge.example.com","port":"443","id":"` + testUUID + `","aid":"0","scy":"auto","net":"ws","path":"/ws","tls":"tls"}`
	text := "vless://" + testUUID + "@edge.example.com:443?security=tls&type=ws&path=%2Fws&host=cdn.example.com#vless\n" +
		"trojan://test-password@edge.example.com:443#trojan\nss://YWVzLTEyOC1nY206dGVzdA@edge.example.com:8388#ss\nsocks5://u:p@127.0.0.1:1080#socks\n" +
		"vmess://" + base64.StdEncoding.EncodeToString([]byte(vmess)) + "\nhysteria2://test@edge.example.com:443#hy2\ntuic://" + testUUID + ":test@edge.example.com:443#tuic\nanytls://test@edge.example.com:443#anytls"
	original, err := ParseText(text)
	if err != nil || len(original.Nodes) != 8 {
		t.Fatal("fixture failed")
	}
	for _, format := range []string{"uri", "base64", "singbox", "clash"} {
		t.Run(format, func(t *testing.T) {
			exported, err := Export(original.Nodes, nil, format)
			if err != nil {
				t.Fatal(err)
			}
			restored, err := ParseText(exported)
			if err != nil || len(restored.Nodes) != 8 || len(restored.Skipped) != 0 {
				t.Fatalf("roundtrip failed: %v skips=%+v", err, restored.Skipped)
			}
			for i, node := range original.Nodes {
				if restored.Nodes[i].ID != node.ID {
					t.Fatalf("node %d changed protocol fields during %s roundtrip", i, format)
				}
			}
		})
	}
}

func TestExportPreservesGroupMembers(t *testing.T) {
	parsed, err := ParseText(`{"outbounds":[{"tag":"one","type":"socks","server":"127.0.0.1","server_port":1080},{"tag":"Proxy","type":"selector","outbounds":["one","DIRECT"]}]}`)
	if err != nil {
		t.Fatal(err)
	}
	for _, format := range []string{"clash", "singbox"} {
		text, err := Export(parsed.Nodes, parsed.Groups, format)
		if err != nil {
			t.Fatal(err)
		}
		result, err := ParseText(text)
		if err != nil || len(result.Groups) != 1 || result.Groups[0].Members[1] != "__direct" {
			t.Fatalf("group roundtrip: %v", err)
		}
	}
}
