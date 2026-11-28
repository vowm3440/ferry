package compile

import (
	"context"
	"encoding/json"
	"testing"

	include "ferry/internal/core/boxinclude"
	"ferry/internal/spec"
	box "github.com/sagernet/sing-box"
	"github.com/sagernet/sing-box/option"
	corejson "github.com/sagernet/sing/common/json"
	"github.com/sagernet/sing/common/json/badoption"
	"github.com/sagernet/sing/service"
)

func TestCompileSplitDNSWithIntranetAndRuleSetsBeforeFakeIP(t *testing.T) {
	s := fixture()
	s.DNS.Mode = "fakeip"
	s.RuleSets = []spec.RuleSet{
		{ID: "apple-cn", Name: "Apple CN", URL: "https://example.com/apple-cn.srs"},
	}
	s.DNS.Servers = []spec.DNSServer{
		{ID: "corp-dns", Name: "Corp DNS", Address: "udp://10.0.0.53", Detour: "direct"},
	}
	s.DNS.Rules = []spec.DNSRule{
		{ID: "d-corp", Name: "Corp", Enabled: true, Domains: []string{"*.corp", "*.local"}, Server: "corp-dns"},
		{ID: "d-cn", Name: "CN", Enabled: true, Domains: []string{"geosite:cn", "ruleset:apple-cn"}, Server: "direct"},
		{ID: "d-off", Name: "Disabled", Enabled: false, Domains: []string{"*.disabled"}, Server: "remote"},
	}
	options, err := Compile(s)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}
	options.Route.RuleSet = []option.RuleSet{
		{Type: "inline", Tag: badoption.Listable[string]{"geosite-cn"}, InlineOptions: option.PlainRuleSet{Rules: []option.HeadlessRule{{Type: "default", DefaultOptions: option.DefaultHeadlessRule{Domain: []string{"example.cn"}}}}}},
		{Type: "inline", Tag: badoption.Listable[string]{"apple-cn"}, InlineOptions: option.PlainRuleSet{Rules: []option.HeadlessRule{{Type: "default", DefaultOptions: option.DefaultHeadlessRule{Domain: []string{"apple.com.cn"}}}}}},
	}
	rawRules, err := corejson.Marshal(options.DNS.Rules)
	if err != nil {
		t.Fatal(err)
	}
	var parsed []map[string]any
	if err := json.Unmarshal(rawRules, &parsed); err != nil {
		t.Fatal(err)
	}
	// Expected: [0]=clash_mode:direct, [1]=corp-dns, [2]=cn direct, [3]=fakeip catch-all
	if len(parsed) != 4 || parsed[1]["server"] != "__dns_custom_corp-dns" || parsed[2]["server"] != "__dns_direct" || parsed[3]["server"] != "__dns_fakeip" {
		t.Fatalf("unexpected DNS rule order (intranet/CN must precede fakeip): %v", parsed)
	}
	ctx := service.ExtendContext(include.Context(context.Background()))
	instance, err := box.New(box.Options{Context: ctx, Options: options})
	if err != nil {
		t.Fatalf("sing-box rejected split DNS options: %v", err)
	}
	if len(instance.Router().Rules()) != len(CompiledRuleMeta(s)) {
		t.Fatalf("router rule count %d != CompiledRuleMeta count %d", len(instance.Router().Rules()), len(CompiledRuleMeta(s)))
	}
	_ = instance.Close()
}

func TestCompileIPv4OnlyOmitsTunAndFakeIPv6(t *testing.T) {
	s := fixture()
	s.DNS.Mode = "fakeip"
	s.DNS.Strategy = "ipv4_only"
	s.Inbound.Tun = spec.Tun{Enabled: true, Stack: "mixed", StrictRoute: true, MTU: 9000}
	options, err := Compile(s)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}
	raw, err := corejson.Marshal(options)
	if err != nil {
		t.Fatal(err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatal(err)
	}
	inbounds := parsed["inbounds"].([]any)
	tun := inbounds[1].(map[string]any)
	if tun["address"] != "172.19.0.1/30" {
		t.Fatalf("expected IPv4-only TUN address, got %v", tun["address"])
	}
	dns := parsed["dns"].(map[string]any)
	servers := dns["servers"].([]any)
	fakeip := servers[len(servers)-1].(map[string]any)
	if _, hasV6 := fakeip["inet6_range"]; hasV6 {
		t.Fatalf("expected no inet6_range in ipv4_only fakeip: %v", fakeip)
	}
	ctx := service.ExtendContext(include.Context(context.Background()))
	instance, err := box.New(box.Options{Context: ctx, Options: options})
	if err != nil {
		t.Fatalf("sing-box rejected ipv4_only TUN config: %v", err)
	}
	_ = instance.Close()
}
