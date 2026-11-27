package routing

import (
	"ferry/internal/spec"
	"testing"
)

func TestImportExtendedClashSingboxPPXSemantics(t *testing.T) {
	cfg := spec.Default()
	clashInput := "RULE-SET,geosite-cn,DIRECT\nDOMAIN-WILDCARD,*.cdn.*.example.com,DIRECT\nPROCESS-NAME-WILDCARD,steam*.exe,DIRECT\nRULE-SET,unknown-set,DIRECT\n"
	clashPreview, err := Import("clash", clashInput, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(clashPreview.Rules) != 3 || len(clashPreview.Warnings) != 1 {
		t.Fatalf("unexpected clash preview: rules=%d warnings=%v", len(clashPreview.Rules), clashPreview.Warnings)
	}
	if clashPreview.Rules[0].Hosts[0] != "geosite:cn" || clashPreview.Rules[1].Hosts[0] != "*.cdn.*.example.com" || clashPreview.Rules[2].Apps[0] != "steam*.exe" {
		t.Fatalf("unexpected clash rules: %+v", clashPreview.Rules)
	}

	sbInput := `{"route":{"rules":[{"domain_keyword":["google"],"outbound":"DIRECT"},{"rule_set":["geoip-cn"],"outbound":"DIRECT"},{"rule_set":["unknown"],"outbound":"DIRECT"}]}}`
	sbPreview, err := Import("singbox", sbInput, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(sbPreview.Rules) != 2 || len(sbPreview.Warnings) != 1 {
		t.Fatalf("unexpected singbox preview: rules=%d warnings=%v", len(sbPreview.Rules), sbPreview.Warnings)
	}
	if sbPreview.Rules[0].Hosts[0] != "*google*" || sbPreview.Rules[1].Hosts[0] != "geoip:cn" {
		t.Fatalf("unexpected singbox rules: %+v", sbPreview.Rules)
	}

	ppxInput := `<RuleList><Rule enabled="true"><Name>WithComment</Name><Comment>notes</Comment><Applications>Any</Applications><Targets>example.com</Targets><Action type="Direct"/></Rule></RuleList>`
	ppxPreview, err := Import("ppx", ppxInput, cfg)
	if err != nil || len(ppxPreview.Rules) != 1 || len(ppxPreview.Rules[0].Apps) != 0 {
		t.Fatalf("unexpected ppx preview: %v %+v", err, ppxPreview)
	}
}
