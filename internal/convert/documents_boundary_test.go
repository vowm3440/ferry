package convert

import "testing"

func TestClashRejectsMalformedAndInapplicableFields(t *testing.T) {
	for _, extra := range []string{`"servername":{},"sni":{}`, `"obfs":"salamander","obfs-password":"secret"`, `"network":3`, `"network":"ws","grpc-opts":{"grpc-service-name":"ignored"}`, `"reality-opts":{"public-key":"q5lP3LTfQLLv27R5maqTkeXn6tOtk5HCThjSpKmuyEM","spider-x":123}`} {
		input := `{"proxies":[{"name":"test","type":"vless","uuid":"` + testUUID + `","server":"edge.example.com","port":443,"tls":true,` + extra + `}]}`
		result, err := ParseText(input)
		if err == nil && len(result.Nodes) != 0 {
			t.Fatal("malformed or inapplicable fields were silently ignored")
		}
	}
	trojanFlow := `{"proxies":[{"name":"bad","type":"trojan","password":"secret","server":"edge.example.com","port":443,"tls":true,"flow":"xtls-rprx-vision"}]}`
	if result, err := ParseText(trojanFlow); err == nil && len(result.Nodes) != 0 {
		t.Fatal("flow accepted on non-vless Clash node")
	}
}
