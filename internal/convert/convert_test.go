package convert

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	"ferry/internal/spec"
)

const testUUID = "11111111-2222-4333-8444-555555555555"

func outbound(t *testing.T, node spec.Node) map[string]any {
	t.Helper()
	var value map[string]any
	if err := json.Unmarshal(node.Outbound, &value); err != nil {
		t.Fatalf("outbound is not JSON: %v", err)
	}
	return value
}

func one(t *testing.T, line string) (spec.Node, map[string]any) {
	t.Helper()
	result, err := ParseText(line)
	if err != nil {
		t.Fatalf("ParseText: %v", err)
	}
	if len(result.Nodes) != 1 || len(result.Skipped) != 0 {
		t.Fatalf("want 1 node and no skips, got %d nodes, skipped %+v", len(result.Nodes), result.Skipped)
	}
	return result.Nodes[0], outbound(t, result.Nodes[0])
}

func TestVLESSWebSocketTLS(t *testing.T) {
	link := "vless://" + testUUID + "@edge.example.com:443?encryption=none&security=tls&sni=sni.example.com" +
		"&insecure=0&allowInsecure=0&type=ws&host=cdn.example.com&path=%2Fws-path#%E7%BE%8E%E5%9B%BD-%E5%8A%A0%E5%B7%9E"
	node, o := one(t, link)
	if node.Tag != "美国-加州" {
		t.Errorf("tag = %q", node.Tag)
	}
	if o["type"] != "vless" || o["server"] != "edge.example.com" || o["server_port"] != float64(443) || o["uuid"] != testUUID {
		t.Errorf("unexpected outbound: %v", o)
	}
	tls := o["tls"].(map[string]any)
	if tls["enabled"] != true || tls["server_name"] != "sni.example.com" {
		t.Errorf("tls = %v", tls)
	}
	if _, set := tls["insecure"]; set {
		t.Errorf("insecure must stay unset when the link says 0")
	}
	transport := o["transport"].(map[string]any)
	headers := transport["headers"].(map[string]any)
	if transport["type"] != "ws" || transport["path"] != "/ws-path" || headers["Host"] != "cdn.example.com" {
		t.Errorf("transport = %v", transport)
	}
	if _, set := o["flow"]; set {
		t.Errorf("flow must be absent")
	}
	full := spec.Default()
	full.Nodes = []spec.Node{node}
	if err := full.Validate(); err != nil {
		t.Errorf("parsed node rejected by RunSpec validation: %v", err)
	}
}

func TestVLESSVariants(t *testing.T) {
	_, plain := one(t, "vless://"+testUUID+"@192.0.2.10:8080")
	if _, set := plain["tls"]; set {
		t.Errorf("no security means no tls: %v", plain)
	}
	node, vision := one(t, "vless://"+testUUID+"@[2001:db8::1]:443?security=tls&flow=xtls-rprx-vision&alpn=h2,http/1.1&allowInsecure=1")
	if vision["server"] != "2001:db8::1" || vision["flow"] != "xtls-rprx-vision" {
		t.Errorf("outbound = %v", vision)
	}
	tls := vision["tls"].(map[string]any)
	if tls["insecure"] != true || len(tls["alpn"].([]any)) != 2 {
		t.Errorf("tls = %v", tls)
	}
	if node.Tag != "2001:db8::1:443" {
		t.Errorf("fallback tag = %q", node.Tag)
	}
}

func TestTrojan(t *testing.T) {
	node, o := one(t, "trojan://p%40ss@t.example.com:443?peer=peer.example.com&type=ws&path=/t#Trojan%20A")
	if node.Tag != "Trojan A" || o["type"] != "trojan" || o["password"] != "p@ss" {
		t.Errorf("node = %+v outbound = %v", node, o)
	}
	tls := o["tls"].(map[string]any)
	if tls["enabled"] != true || tls["server_name"] != "peer.example.com" {
		t.Errorf("tls = %v", tls)
	}
	if o["transport"].(map[string]any)["path"] != "/t" {
		t.Errorf("transport = %v", o["transport"])
	}
}

func TestShadowsocks(t *testing.T) {
	user := base64.RawURLEncoding.EncodeToString([]byte("aes-256-gcm:secret:with:colon"))
	_, sip := one(t, "ss://"+user+"@ss.example.com:8388/?#SS")
	if sip["method"] != "aes-256-gcm" || sip["password"] != "secret:with:colon" || sip["server_port"] != float64(8388) {
		t.Errorf("sip002 = %v", sip)
	}
	legacy := base64.StdEncoding.EncodeToString([]byte("chacha20-ietf-poly1305:pw@198.51.100.7:443"))
	node, old := one(t, "ss://"+legacy+"#old")
	if old["method"] != "chacha20-ietf-poly1305" || old["password"] != "pw" || old["server"] != "198.51.100.7" || node.Tag != "old" {
		t.Errorf("legacy = %v tag=%q", old, node.Tag)
	}
	_, plain := one(t, "ss://aes-128-gcm:pw%3A1@ss.example.com:1234")
	if plain["method"] != "aes-128-gcm" || plain["password"] != "pw:1" {
		t.Errorf("plain userinfo = %v", plain)
	}
}

func TestSocks(t *testing.T) {
	_, open := one(t, "socks5://127.0.0.1:1080#local")
	if open["type"] != "socks" || open["version"] != "5" {
		t.Errorf("socks = %v", open)
	}
	if _, set := open["username"]; set {
		t.Errorf("no credentials expected: %v", open)
	}
	_, auth := one(t, "socks://"+base64.StdEncoding.EncodeToString([]byte("user:pw"))+"@s.example.com:1080")
	if auth["username"] != "user" || auth["password"] != "pw" {
		t.Errorf("base64 userinfo = %v", auth)
	}
	_, direct := one(t, "socks5://u:p@s.example.com:1080")
	if direct["username"] != "u" || direct["password"] != "p" {
		t.Errorf("plain userinfo = %v", direct)
	}
}

func TestSkipsWithReasonsAndNoSecrets(t *testing.T) {
	cases := map[string]string{
		"vless://" + testUUID + "@a.example.com:443?security=reality&pbk=KEYKEY":                                   "支持的范围",
		"vless://" + testUUID + "@a.example.com:443?security=tls&fp=unsupported":                                   "支持的范围",
		"vless://" + testUUID + "@a.example.com:443?type=unsupported":                                              "传输方式",
		"vless://" + testUUID + "@a.example.com:443?encryption=aes-128-gcm":                                        "加密",
		"vless://not-a-uuid@a.example.com:443":                                                                     "支持的范围",
		"vless://" + testUUID + "@a.example.com":                                                                   "端口",
		"vless://" + testUUID + "@a.example.com:443?headerType=http":                                               "headerType",
		"trojan://@a.example.com:443":                                                                              "密码",
		"trojan://pw@a.example.com:443?security=none":                                                              "传输安全",
		"ss://" + base64.RawURLEncoding.EncodeToString([]byte("rc4-md5:pw")) + "@a.b.com:1":                        "支持的范围",
		"ss://" + base64.RawURLEncoding.EncodeToString([]byte("aes-128-gcm:pw")) + "@a.b.com:1/?plugin=obfs-local": "插件",
		"ss://%%%":                 "无法解析",
		"vmess://eyJhZGQiOiJhIn0=": "vmess",
		"hysteria2://pw@a.example.com:443?obfs=invalid": "混淆",
		"just some words":                       "无法识别",
		"vless://" + testUUID + "@bad host:443": "无法解析",
	}
	for line, want := range cases {
		result, err := ParseText("socks5://127.0.0.1:1\n" + line)
		if err != nil {
			t.Errorf("%q: %v", line, err)
			continue
		}
		if len(result.Nodes) != 1 || len(result.Skipped) != 1 {
			t.Errorf("%q: nodes=%d skipped=%+v", line, len(result.Nodes), result.Skipped)
			continue
		}
		skip := result.Skipped[0]
		if skip.Line != 2 || !strings.Contains(skip.Reason, want) {
			t.Errorf("%q: skip = %+v, want reason containing %q", line, skip, want)
		}
		for _, secret := range []string{testUUID, "KEYKEY", "pw"} {
			if strings.Contains(skip.Reason, secret) {
				t.Errorf("%q: reason leaks %q: %s", line, secret, skip.Reason)
			}
		}
	}
}

func TestBase64SubscriptionBodyAndDedupe(t *testing.T) {
	body := "socks5://127.0.0.1:1080#a\r\n\r\nsocks5://127.0.0.1:1080#duplicate\nsocks5://127.0.0.1:1081#b\n"
	for _, encoded := range []string{
		base64.StdEncoding.EncodeToString([]byte(body)),
		base64.RawURLEncoding.EncodeToString([]byte(body)),
		" " + base64.StdEncoding.EncodeToString([]byte(body))[:20] + "\n" + base64.StdEncoding.EncodeToString([]byte(body))[20:] + " ",
	} {
		result, err := ParseText(encoded)
		if err != nil {
			t.Fatalf("ParseText: %v", err)
		}
		if len(result.Nodes) != 2 || result.Nodes[0].Tag != "a" || result.Nodes[1].Tag != "b" {
			t.Errorf("nodes = %+v", result.Nodes)
		}
		if len(result.Skipped) != 1 || result.Skipped[0].Line != 3 || !strings.Contains(result.Skipped[0].Reason, "重复") {
			t.Errorf("skipped = %+v", result.Skipped)
		}
	}
}

func TestStableIDsAndLabels(t *testing.T) {
	first, _ := one(t, "socks5://127.0.0.1:1080#name-one")
	second, _ := one(t, "socks5://127.0.0.1:1080#name-two")
	other, _ := one(t, "socks5://127.0.0.1:1081#name-one")
	if first.ID != second.ID || first.ID == other.ID || !strings.HasPrefix(first.ID, "n-") {
		t.Errorf("ids: %q %q %q", first.ID, second.ID, other.ID)
	}
	long, _ := one(t, "socks5://127.0.0.1:1080#"+strings.Repeat("节", 300))
	if len(long.Tag) > 256 || !strings.HasPrefix(long.Tag, "节节") {
		t.Errorf("long tag length = %d", len(long.Tag))
	}
	control, _ := one(t, "socks5://127.0.0.1:1080#a%0Ab%09c")
	if control.Tag != "abc" {
		t.Errorf("control characters must be removed: %q", control.Tag)
	}
}

func TestRejectsUnusableInput(t *testing.T) {
	for _, text := range []string{"", "   \n  ", "hello world", "aGVsbG8gd29ybGQ=", strings.Repeat("a", MaxBytes+1)} {
		if _, err := ParseText(text); err == nil {
			t.Errorf("%.20q: expected an error", text)
		}
	}
	var many strings.Builder
	for i := 0; i <= spec.MaxNodes; i++ {
		many.WriteString("socks5://127.0.0.1:" + itoa(i+1) + "\n")
	}
	if _, err := ParseText(many.String()); err == nil || !strings.Contains(err.Error(), "数量") {
		t.Errorf("too many nodes: err = %v", err)
	}
}

func itoa(value int) string {
	data, _ := json.Marshal(value)
	return string(data)
}
