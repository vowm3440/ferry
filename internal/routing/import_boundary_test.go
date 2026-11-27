package routing

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"ferry/internal/spec"
)

func TestImportRejectsLiteralConditionsThatWouldBecomePatterns(t *testing.T) {
	for _, input := range []string{
		"DOMAIN,203.0.113.1,DIRECT", "DOMAIN-SUFFIX,203.0.113.1,DIRECT",
		"PROCESS-NAME,*.exe,DIRECT", "NETWORK,any,DIRECT", "IP-CIDR6,192.0.2.0/24,DIRECT",
	} {
		preview, err := Import("clash", input, importSpec())
		if err != nil || len(preview.Rules) != 0 || len(preview.Warnings) != 1 {
			t.Fatalf("condition silently changed: %+v %v", preview, err)
		}
	}
	for _, input := range []string{
		`[{"domain":"203.0.113.1","outbound":"n1"}]`,
		`[{"network":["tcp","udp"],"outbound":"n1"}]`,
	} {
		preview, err := Import("singbox", input, importSpec())
		if err != nil || len(preview.Rules) != 0 || len(preview.Warnings) != 1 {
			t.Fatalf("condition silently changed: %+v %v", preview, err)
		}
	}
}

func TestImportStableAcrossSingboxObjectKeyOrder(t *testing.T) {
	first := `[{"domain":"x.example","ip_cidr":"192.0.2.0/24","port":443,"port_range":"8000:9000","outbound":"n1"}]`
	second := `[{"port_range":"8000:9000","outbound":"n1","port":443,"ip_cidr":"192.0.2.0/24","domain":"x.example"}]`
	left, err := Import("singbox", first, importSpec())
	if err != nil {
		t.Fatal(err)
	}
	for range 20 {
		right, err := Import("singbox", second, importSpec())
		if err != nil || !reflect.DeepEqual(left, right) {
			t.Fatal("object key order changes preview or ID")
		}
	}
}

func TestImportRejectsAmbiguousTargetWithoutLeakingName(t *testing.T) {
	s := importSpec()
	s.Nodes = append(s.Nodes, spec.Node{ID: "n2", Tag: "Tokyo"})
	preview, err := Import("clash", "DOMAIN,x.test,Tokyo", s)
	if err != nil || len(preview.Rules) != 0 || len(preview.Warnings) != 1 {
		t.Fatalf("ambiguous target accepted: %+v %v", preview, err)
	}
}

func TestPPXStructureAndTokenLimits(t *testing.T) {
	for _, input := range []string{
		`<RuleList enabled="false"><Rule><Action type="Direct"/></Rule></RuleList>`,
		`<RuleList>SECRET<Rule><Action type="Direct"/></Rule></RuleList>`,
		`<RuleList><Rule enabled="true" enabled="false"/></RuleList>`,
		`<RuleList/><RuleList/>`, `<RuleList>&SECRET;</RuleList>`,
		"<RuleList>" + strings.Repeat("<!--x-->", 50001) + "</RuleList>",
		"<RuleList>" + strings.Repeat("<Rule><Action type=\"Direct\"/></Rule>", spec.MaxRules+1) + "</RuleList>",
	} {
		if _, err := Import("ppx", input, importSpec()); err == nil || strings.Contains(fmt.Sprint(err), "SECRET") {
			t.Fatalf("PPX boundary failed: %v", err)
		}
	}
}
