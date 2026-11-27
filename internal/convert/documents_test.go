package convert

import "testing"

func TestClashDocumentPreservesGroupsAndTLS(t *testing.T) {
	input := `proxies:
  - name: vless-node
    type: vless
    server: edge.example.com
    port: 443
    uuid: 11111111-2222-4333-8444-555555555555
    tls: true
    servername: edge.example.com
    network: ws
    ws-opts:
      path: /ws
      headers:
        Host: cdn.example.com
proxy-groups:
  - name: Proxy
    type: select
    proxies: [vless-node, DIRECT]
`
	result, err := ParseText(input)
	if err != nil || len(result.Nodes) != 1 || len(result.Groups) != 1 {
		t.Fatalf("document: %v", err)
	}
	if result.Groups[0].Members[0] != result.Nodes[0].ID || result.Groups[0].Members[1] != "__direct" {
		t.Fatal("group references not preserved")
	}
	o := outbound(t, result.Nodes[0])
	if o["tls"].(map[string]any)["server_name"] != "edge.example.com" {
		t.Fatal("TLS server name lost")
	}
}

func TestSingboxAndSIP008Documents(t *testing.T) {
	for _, input := range []string{
		`{"outbounds":[{"tag":"node","type":"socks","server":"127.0.0.1","server_port":1080},{"type":"selector","tag":"Proxy","outbounds":["node","direct"]},{"type":"direct","tag":"direct"}]}`,
		`{"version":1,"servers":[{"remarks":"node","server":"edge.example.com","server_port":8388,"method":"aes-128-gcm","password":"synthetic"}]}`,
	} {
		result, err := ParseText(input)
		if err != nil || len(result.Nodes) != 1 {
			t.Fatalf("document import: %v", err)
		}
	}
}

func TestRejectsYAMLAliasesAndUnknownNodeFields(t *testing.T) {
	if _, err := ParseText("proxies: &all [*all]"); err == nil {
		t.Fatal("recursive YAML alias accepted")
	}
	result, err := ParseText(`{"outbounds":[{"type":"socks","tag":"unsafe","server":"127.0.0.1","server_port":1080,"detour":"foreign"}]}`)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Nodes) != 0 || len(result.Skipped) != 1 {
		t.Fatal("unsupported outbound field silently discarded")
	}
}
