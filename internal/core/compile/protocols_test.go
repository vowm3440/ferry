//go:build with_quic

package compile

import (
	"context"
	"encoding/base64"
	"ferry/internal/convert"
	include "ferry/internal/core/boxinclude"
	"ferry/internal/spec"
	box "github.com/sagernet/sing-box"
	"testing"
)

func TestImportedProtocolsConstructWithRealCore(t *testing.T) {
	uuid := "11111111-2222-4333-8444-555555555555"
	vmess := `{"v":"2","add":"edge.example.com","port":"443","id":"` + uuid + `","aid":"0","net":"ws","path":"/ws","tls":"tls"}`
	for _, link := range []string{
		"vmess://" + base64.StdEncoding.EncodeToString([]byte(vmess)),
		"vless://" + uuid + "@edge.example.com:443?security=tls&type=grpc&serviceName=test",
		"hysteria2://synthetic@edge.example.com:443",
		"tuic://" + uuid + ":synthetic@edge.example.com:443",
		"anytls://synthetic@edge.example.com:443",
	} {
		parsed, err := convert.ParseText(link)
		if err != nil || len(parsed.Nodes) != 1 {
			t.Fatal("fixture parse failed")
		}
		config := spec.Default()
		config.Nodes = parsed.Nodes
		options, err := Compile(config)
		if err != nil {
			t.Fatal(err)
		}
		instance, err := box.New(box.Options{Options: options, Context: include.Context(context.Background())})
		if err != nil {
			t.Fatal("converted protocol rejected by core constructor")
		}
		if err := instance.Close(); err != nil {
			t.Fatal(err)
		}
	}
}
