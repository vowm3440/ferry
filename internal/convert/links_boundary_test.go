package convert

import (
	"encoding/json"
	"strings"
	"testing"

	"ferry/internal/spec"
)

func TestRejectsAmbiguousOrLossyShareLinks(t *testing.T) {
	vless := "vless://" + testUUID + "@edge.example.com:443"
	cases := map[string]string{
		"duplicate security":             vless + "?security=none&security=tls",
		"duplicate type":                 vless + "?type=ws&type=tcp",
		"duplicate unknown":              vless + "?unused=&unused=secret",
		"malformed escape":               vless + "?security=%zz",
		"invalid separator":              vless + "?security=none;security=tls",
		"ignored path":                   vless + "/secret",
		"ignored user password":          "vless://" + testUUID + ":secret@edge.example.com:443",
		"ignored trojan password suffix": "trojan://part:secret@edge.example.com:443",
		"disabled tls with sni":          vless + "?security=none&sni=secret.example.com",
		"disabled tls with alpn":         vless + "?alpn=h2",
		"disabled tls with insecure":     vless + "?insecure=1",
		"invalid tls flag":               vless + "?security=tls&insecure=secret",
		"conflicting tls flags":          vless + "?security=tls&insecure=0&allowInsecure=1",
		"conflicting tls names":          vless + "?security=tls&sni=a.example.com&peer=b.example.com",
		"spx without reality":            vless + "?security=tls&sni=edge.example.com&spx=%2F",
		"spx with control char":          vless + "?security=reality&sni=edge.example.com&pbk=q5lP3LTfQLLv27R5maqTkeXn6tOtk5HCThjSpKmuyEM&spx=%2F%0Asecret",
		"grpc unsupported mode":          vless + "?security=tls&type=grpc&serviceName=svc&mode=multi",
		"mode on non-grpc":               vless + "?security=tls&type=ws&path=%2Fws&mode=gun",
		"duplicate shadowsocks plugin":   "ss://aes-128-gcm:secret@edge.example.com:443?plugin=&plugin=obfs",
		"malformed socks query":          "socks5://127.0.0.1:1080?x=%zz",
		"ignored socks path":             "socks5://127.0.0.1:1080/secret",
	}
	for name, link := range cases {
		t.Run(name, func(t *testing.T) {
			result, err := ParseText(link)
			if err != nil {
				t.Fatal(err)
			}
			if len(result.Nodes) != 0 || len(result.Skipped) != 1 || result.Skipped[0].Reason == "" {
				t.Fatal("link imported with parameters silently dropped or changed")
			}
			for _, secret := range []string{testUUID, "secret", "edge.example.com"} {
				if strings.Contains(result.Skipped[0].Reason, secret) {
					t.Fatal("skip reason exposes link content")
				}
			}
		})
	}
}

func TestFallbackLabelFitsRunSpecWithoutChangingServer(t *testing.T) {
	host := strings.Repeat("a", 63) + "." + strings.Repeat("b", 63) + "." + strings.Repeat("c", 63) + "." + strings.Repeat("d", 61)
	node, value := one(t, "socks5://"+host+":1080")
	config := spec.Default()
	config.Nodes = []spec.Node{node}
	if err := config.Validate(); err != nil {
		t.Fatalf("valid server produced an invalid display label: %v", err)
	}
	if value["server"] != host {
		t.Fatal("server address was shortened together with the label")
	}
}

func FuzzShareLinksProduceValidNodes(f *testing.F) {
	for _, input := range []string{
		"socks5://127.0.0.1:1080#local", "trojan://test@edge.example.com:443",
		"vless://" + testUUID + "@edge.example.com:443?security=tls&type=ws&path=%2Fws",
		"ss://YWVzLTEyOC1nY206dGVzdA@edge.example.com:443",
		"vless://" + testUUID + "@edge.example.com:443?security=none&security=tls",
		"c29ja3M1Oi8vMTI3LjAuMC4xOjEwODA=", "\xff\x00://?%zz",
	} {
		f.Add(input)
	}
	f.Fuzz(func(t *testing.T, input string) {
		result, err := ParseText(input)
		if err != nil {
			return
		}
		for _, node := range result.Nodes {
			config := spec.Default()
			config.Nodes = []spec.Node{node}
			data, err := json.Marshal(config)
			if err != nil {
				t.Fatal("converted node cannot be encoded")
			}
			if _, err := spec.Decode(data); err != nil {
				t.Fatalf("converted node cannot cross the core boundary: %v", err)
			}
		}
	})
}

func TestEquivalentTLSAliasesAndEncodedTrojanPassword(t *testing.T) {
	_, value := one(t, "trojan://part%3Asecret@edge.example.com:443/?security=tls&insecure=0&allowInsecure=false&sni=edge.example.com&peer=edge.example.com")
	if value["password"] != "part:secret" {
		t.Fatal("percent-encoded password was changed")
	}
	tls := value["tls"].(map[string]any)
	if tls["insecure"] == true || tls["server_name"] != "edge.example.com" {
		t.Fatal("equivalent TLS aliases changed settings")
	}
}
