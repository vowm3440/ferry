package convert

import (
	"encoding/base64"
	"net/url"
	"strings"
	"testing"
)

func testWGKey(seed byte) string {
	var raw [32]byte
	for i := range raw {
		raw[i] = seed + byte(i)
	}
	return base64.StdEncoding.EncodeToString(raw[:])
}

func TestWireGuardURIClashSingboxRoundtrip(t *testing.T) {
	priv := testWGKey(1)
	pub := testWGKey(40)
	psk := testWGKey(80)
	link := "wireguard://" + url.QueryEscape(priv) + "@wg.example.com:51820?publickey=" + url.QueryEscape(pub) +
		"&presharedkey=" + url.QueryEscape(psk) + "&address=10.0.0.2%2F32,fd00%3A%3A2%2F128&reserved=1,2,3&mtu=1420#wg-home"
	original, err := ParseText(link)
	if err != nil || len(original.Nodes) != 1 || len(original.Skipped) != 0 {
		t.Fatalf("wireguard URI parse failed: %v skips=%+v", err, original.Skipped)
	}
	for _, format := range []string{"uri", "base64", "clash", "singbox"} {
		t.Run(format, func(t *testing.T) {
			exported, err := Export(original.Nodes, nil, format)
			if err != nil {
				t.Fatalf("export %s failed: %v", format, err)
			}
			restored, err := ParseText(exported)
			if err != nil || len(restored.Nodes) != 1 || len(restored.Skipped) != 0 {
				t.Fatalf("restore %s failed: %v skips=%+v", format, err, restored.Skipped)
			}
			if restored.Nodes[0].ID != original.Nodes[0].ID {
				t.Fatalf("wireguard %s roundtrip altered canonical node ID", format)
			}
		})
	}
}

func TestWireGuardRejectsInvalidFields(t *testing.T) {
	priv := url.QueryEscape(testWGKey(1))
	pub := url.QueryEscape(testWGKey(2))
	bad := []string{
		"wireguard://@wg.example.com:51820?publickey=" + pub + "&address=10.0.0.2/32",
		"wireguard://notbase64@wg.example.com:51820?publickey=" + pub + "&address=10.0.0.2/32",
		"wireguard://" + priv + "@wg.example.com:51820?publickey=short&address=10.0.0.2/32",
		"wireguard://" + priv + "@wg.example.com:51820?publickey=" + pub + "&address=not-an-ip",
		"wireguard://" + priv + "@wg.example.com:51820?publickey=" + pub + "&address=10.0.0.2/32&reserved=1,2",
		"wireguard://" + priv + "@wg.example.com:51820?publickey=" + pub + "&address=10.0.0.2/32&reserved=1,2,300",
		"wireguard://" + priv + "@wg.example.com:51820?publickey=" + pub + "&address=10.0.0.2/32&mtu=500",
		"wireguard://" + priv + "@wg.example.com:51820?publickey=" + pub + "&address=10.0.0.2/32&unknown=1",
	}
	for _, line := range bad {
		res, err := ParseText(line)
		if err == nil && len(res.Nodes) > 0 {
			t.Fatalf("expected invalid wireguard link to be rejected: %s", line)
		}
	}
	if _, err := ParseText(`{"endpoints":[{"type":"wireguard","tag":"bad","address":["10.0.0.2/32"],"private_key":"` + testWGKey(1) + `","peers":[]}]}`); err == nil {
		// Document with 0 valid nodes succeeds with skipped entries
	}
	res, err := ParseText(`{"endpoints":[{"type":"wireguard","tag":"bad","address":["10.0.0.2/32"],"private_key":"` + testWGKey(1) + `","peers":[{"address":"wg.example.com","port":51820,"public_key":"` + testWGKey(2) + `","unknown_field":true}]}]}`)
	if err != nil || len(res.Nodes) != 0 || len(res.Skipped) != 1 || !strings.Contains(res.Skipped[0].Reason, "不支持") {
		t.Fatalf("expected unknown peer field to be skipped: %v %+v", err, res)
	}
	clashBad := "proxies:\n  - { name: bad-wg, type: wireguard, server: wg.example.com, port: 51820, private-key: " + testWGKey(1) + ", public-key: " + testWGKey(2) + " }\n"
	clashRes, err := ParseText(clashBad)
	if err != nil || len(clashRes.Nodes) != 0 || len(clashRes.Skipped) != 1 {
		t.Fatalf("expected clash wireguard missing ip to be skipped: %v %+v", err, clashRes)
	}
	twoIPv4, err := ParseText("wireguard://" + priv + "@wg.example.com:51820?publickey=" + pub + "&address=10.0.0.2/32,10.0.0.3/32#wg2")
	if err != nil || len(twoIPv4.Nodes) != 1 {
		t.Fatalf("expected two IPv4 addresses to parse: %v", err)
	}
	if _, err := Export(twoIPv4.Nodes, nil, "clash"); err == nil {
		t.Fatal("expected clash export to reject duplicate IPv4 addresses")
	}
}
