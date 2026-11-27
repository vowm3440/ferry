package routing

import (
	"encoding/json"
	"strings"
	"testing"

	"ferry/internal/spec"
)

func TestAppGroupsCompileToProcessAlternatives(t *testing.T) {
	s := spec.Default()
	s.AppGroups = []spec.AppGroup{{ID: "browsers", Name: "浏览器", Apps: []string{"chrome.exe", `C:\Apps (x86)\a+b?.exe`}}}
	r := spec.Rule{ID: "r", Name: "应用分组", Enabled: true, Network: "tcp", Action: "direct",
		Apps: []string{"@group:browsers", "edge.exe"}, Hosts: []string{"example.com"}, Ports: []string{"443"}}
	value, err := Build(s, r)
	if err != nil {
		t.Fatal(err)
	}
	matchCompiled(t, value, []matchCase{
		{`D:\Apps\CHROME.EXE`, "example.com", "tcp", 443, true},
		{`C:\Apps (x86)\a+b1.exe`, "example.com", "tcp", 443, true},
		{`C:\Apps\edge.exe`, "example.com", "tcp", 443, true},
		{`C:\Apps\other.exe`, "example.com", "tcp", 443, false},
		{`C:\Apps x86\ab1.exe`, "example.com", "tcp", 443, false},
		{`C:\Apps (x86)\a+b12.exe`, "example.com", "tcp", 443, false},
		{`C:\Apps\chrome.exe`, "example.com", "udp", 443, false},
		{`C:\Apps\chrome.exe`, "other.com", "tcp", 443, false},
		{`C:\Apps\chrome.exe`, "example.com", "tcp", 80, false},
	})
	data, _ := json.Marshal(value)
	if strings.Contains(string(data), "@group:") || strings.Contains(string(data), "浏览器") {
		t.Fatal("group metadata escaped into the compiled process matcher")
	}
}

func TestResolveAppGroupsPreserveProcessPreconditions(t *testing.T) {
	s := spec.Default()
	s.AppGroups = []spec.AppGroup{{ID: "browsers", Name: "浏览器", Apps: []string{"chrome.exe", "firefox.exe"}}}
	r := spec.Rule{ID: "r", Name: "先解析", Enabled: true, Network: "tcp", Action: "direct", ResolveBefore: true,
		Apps: []string{"@group:browsers", "edge.exe"}, Hosts: []string{"example.com", "203.0.113.0/24"}, Ports: []string{"443"}}
	value, err := BuildResolve(s, r)
	if err != nil || value["server"] != "__dns_direct" {
		t.Fatalf("resolve: %v, %v", value, err)
	}
	matchCompiled(t, value, []matchCase{
		{`C:\Apps\chrome.exe`, "unresolved.example", "tcp", 443, true},
		{`C:\Apps\firefox.exe`, "unresolved.example", "tcp", 443, true},
		{`C:\Apps\edge.exe`, "unresolved.example", "tcp", 443, true},
		{`C:\Apps\other.exe`, "unresolved.example", "tcp", 443, false},
		{`C:\Apps\chrome.exe`, "example.com", "tcp", 443, false},
		{`C:\Apps\chrome.exe`, "unresolved.example", "udp", 443, false},
	})
}

func TestAppGroupBuildRejectsMalformedReferencedGroups(t *testing.T) {
	r := spec.Rule{ID: "r", Name: "引用", Enabled: true, Network: "any", Action: "block", ResolveBefore: true,
		Apps: []string{"@group:g"}, Hosts: []string{"203.0.113.0/24"}}
	for _, groups := range [][]spec.AppGroup{
		{}, {{ID: "g", Name: "空", Apps: []string{}}},
		{{ID: "g", Name: "嵌套", Apps: []string{"@group:g"}}},
		{{ID: "g", Name: "语法", Apps: []string{"regex:SECRET"}}},
		{{ID: "g", Name: "重复", Apps: []string{"a.exe"}}, {ID: "g", Name: "重复", Apps: []string{"b.exe"}}},
	} {
		s := spec.Default()
		s.AppGroups = groups
		for _, build := range []func(spec.RunSpec, spec.Rule) (map[string]any, error){Build, BuildResolve} {
			if _, err := build(s, r); err == nil || strings.Contains(err.Error(), "SECRET") {
				t.Fatalf("unsafe referenced group accepted or leaked: %v", err)
			}
		}
	}
}
