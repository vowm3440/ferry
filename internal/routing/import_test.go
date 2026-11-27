package routing

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"ferry/internal/spec"
)

func importSpec() spec.RunSpec {
	s := spec.Default()
	s.Nodes = []spec.Node{{ID: "n1", Tag: "Tokyo"}}
	s.Groups = []spec.Group{{ID: "g1", Name: "Auto", Members: []string{"n1"}}}
	return s
}

func TestImportClashFormatsAndTargetMapping(t *testing.T) {
	lines := []string{"DOMAIN,example.com,DIRECT", "DOMAIN-SUFFIX,example.net,Tokyo", "DOMAIN-KEYWORD,assets,Auto",
		"IP-CIDR,203.0.113.0/24,REJECT,no-resolve", "IP-CIDR6,2001:db8::/32,DIRECT", "DST-PORT,443,Tokyo",
		"PROCESS-NAME,chrome.exe,DIRECT", `PROCESS-PATH,C:\Apps\a.exe,Tokyo`, "NETWORK,UDP,Auto", "MATCH,Auto"}
	data, _ := json.Marshal(lines)
	for _, input := range []string{string(data), "# Exported rules\nrules:\n  - " + strings.Join(lines, "\n  - "), strings.Join(lines, "\n")} {
		preview, err := Import("clash", input, importSpec())
		if err != nil || len(preview.Rules) != 10 || len(preview.Warnings) != 1 {
			t.Fatalf("import: %+v, %v", preview, err)
		}
		if preview.Rules[1].TargetID != "n1" || preview.Rules[2].TargetID != "g1" || preview.Rules[3].ResolveBefore || preview.Rules[8].Network != "udp" {
			t.Fatalf("fields changed: %+v", preview.Rules)
		}
		if !reflect.DeepEqual(preview.Rules[1].Hosts, []string{"example.net", "*.example.net"}) {
			t.Fatal("suffix apex omitted")
		}
		for _, r := range preview.Rules {
			if r.Pinned || r.ID == "" || importSpec().ValidateRule(r) != nil {
				t.Fatalf("invalid imported rule: %+v", r)
			}
		}
		repeated, err := Import("clash", input, importSpec())
		if err != nil || !reflect.DeepEqual(repeated, preview) {
			t.Fatal("preview IDs are not stable")
		}
	}
}

func TestImportClashSkipsWholeUnsupportedRules(t *testing.T) {
	input := "DOMAIN,ok.example,DIRECT\nDOMAIN,bad.example,Tokyo,unknown\nGEOIP,CN,DIRECT\nDOMAIN,secret.example,SECRET\nMATCH,DIRECT,extra\nNETWORK,icmp,DIRECT\nDOMAIN-KEYWORD,foo*bar,DIRECT"
	preview, err := Import("clash", input, importSpec())
	if err != nil || len(preview.Rules) != 1 || len(preview.Warnings) != 6 {
		t.Fatalf("partial rules accepted: %+v, %v", preview, err)
	}
	if strings.Contains(strings.Join(preview.Warnings, " "), "SECRET") {
		t.Fatal("raw credential in warning")
	}
}

func TestImportSingboxFieldsAndRejectSemanticLoss(t *testing.T) {
	input := `{"route":{"rules":[
		{"domain":["example.com"],"domain_suffix":["example.net"],"ip_cidr":["203.0.113.0/24"],"port":[443],"port_range":["8000:9000"],"network":"tcp","action":"route","outbound":"n1"},
		{"process_name":["chrome.exe"],"action":"route","outbound":"g1"},
		{"process_path":["C:\\Apps\\a.exe"],"action":"reject"},
		{"domain":"bad.example","invert":true,"outbound":"n1"},
		{"type":"logical","mode":"or","rules":[{"domain":"x"}],"outbound":"n1"},
		{"process_name":"a.exe","process_path":"C:\\Apps\\a.exe","outbound":"n1"},
		{"domain":"bad.example","action":"sniff"},
		{"domain":"bad.example","outbound":"SECRET"}
	]}}`
	preview, err := Import("singbox", input, importSpec())
	if err != nil || len(preview.Rules) != 3 || len(preview.Warnings) != 7 {
		t.Fatalf("preview: %+v %v", preview, err)
	}
	if !reflect.DeepEqual(preview.Rules[0].Ports, []string{"443", "8000-9000"}) || preview.Rules[0].ResolveBefore {
		t.Fatal("port or resolve semantics changed")
	}
	if preview.Rules[1].TargetID != "g1" || preview.Rules[2].Action != "block" {
		t.Fatal("action mapping changed")
	}
	if !strings.Contains(strings.Join(preview.Warnings, " "), "大小写") {
		t.Fatal("process case conversion not disclosed")
	}
}

func TestImportPPXTypicalRulesAndUnknownFields(t *testing.T) {
	input := `<?xml version="1.0"?><ProxifierProfile version="101"><RuleList>
	<Rule enabled="true"><Name>Browser</Name><Applications>chrome.exe; C:\Apps\a.exe</Applications><Targets>*.example.com;203.0.113.0/24</Targets><Ports>443;8000-9000</Ports><Action type="Proxy">n1</Action></Rule>
	<Rule enabled="false"><Name>Disabled</Name><Action type="Block"/></Rule>
	<Rule enabled="true"><Name>Default</Name><Targets>*</Targets><Action type="Direct"/></Rule>
	<Rule enabled="true"><Name>Unknown proxy</Name><Action type="Proxy">SECRET</Action></Rule>
	<Rule enabled="true"><Name>Unknown feature</Name><Options>1</Options><Action type="Direct"/></Rule>
	</RuleList></ProxifierProfile>`
	preview, err := Import("ppx", input, importSpec())
	if err != nil || len(preview.Rules) != 3 || len(preview.Warnings) != 3 {
		t.Fatalf("preview: %+v, %v", preview, err)
	}
	if len(preview.Rules[0].Apps) != 2 || preview.Rules[1].Enabled || len(preview.Rules[2].Hosts) != 0 {
		t.Fatal("PPX fields changed")
	}
	if strings.Contains(strings.Join(preview.Warnings, " "), "SECRET") {
		t.Fatal("raw proxy reference leaked")
	}
}

func TestImportRejectsBoundariesWithoutEchoingInput(t *testing.T) {
	for _, tc := range []struct{ kind, input string }{
		{"unknown", "SECRET"}, {"clash", strings.Repeat("x", spec.MaxBytes+1)},
		{"clash", strings.Repeat("DOMAIN,x.test,DIRECT\n", spec.MaxRules+1)},
		{"clash", "rules: &SECRET [DOMAIN,x.test,DIRECT]\nother: *SECRET"},
		{"clash", "rules: []\nrules: []"}, {"clash", "rules: []\n---\nrules: []"},
		{"clash", strings.Repeat("[", 30) + "SECRET" + strings.Repeat("]", 30)},
		{"singbox", `{"rules":[{"domain":"SECRET","domain":"x","outbound":"n1"}]}`},
		{"ppx", `<!DOCTYPE x [<!ENTITY SECRET SYSTEM "file:///private">]><RuleList/>`},
		{"ppx", strings.Repeat("<SECRET>", 30) + strings.Repeat("</SECRET>", 30)},
		{"ppx", `<RuleList><Rule><Name>SECRET</Name></RuleList>`},
	} {
		if _, err := Import(tc.kind, tc.input, importSpec()); err == nil || strings.Contains(err.Error(), "SECRET") {
			t.Fatalf("boundary failed for %s: %v", tc.kind, err)
		}
	}
}
