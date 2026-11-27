//go:build with_wireguard

package compile

import (
	"context"
	"encoding/base64"
	"ferry/internal/convert"
	include "ferry/internal/core/boxinclude"
	"ferry/internal/spec"
	box "github.com/sagernet/sing-box"
	"net/url"
	"testing"
)

func TestWireGuardConstructsWithRealCore(t *testing.T) {
	var rawPriv, rawPub [32]byte
	for i := range rawPriv {
		rawPriv[i] = byte(i + 1)
		rawPub[i] = byte(i + 41)
	}
	link := "wireguard://" + url.QueryEscape(base64.StdEncoding.EncodeToString(rawPriv[:])) +
		"@wg.example.com:51820?publickey=" + url.QueryEscape(base64.StdEncoding.EncodeToString(rawPub[:])) +
		"&address=10.0.0.2%2F32&mtu=1420#wg"
	parsed, err := convert.ParseText(link)
	if err != nil || len(parsed.Nodes) != 1 {
		t.Fatalf("wireguard parse failed: %v", err)
	}
	cfg := spec.Default()
	cfg.Nodes = parsed.Nodes
	options, err := Compile(cfg)
	if err != nil {
		t.Fatalf("wireguard compile failed: %v", err)
	}
	if len(options.Endpoints) != 1 {
		t.Fatalf("expected 1 wireguard endpoint, got %d", len(options.Endpoints))
	}
	inst, err := box.New(box.Options{Options: options, Context: include.Context(context.Background())})
	if err != nil {
		t.Fatalf("wireguard rejected by sing-box constructor: %v", err)
	}
	if err := inst.Close(); err != nil {
		t.Fatal(err)
	}
}
