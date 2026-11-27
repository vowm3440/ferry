package spec

import (
	"encoding/json"
	"strings"
	"testing"
)

func sample() RunSpec {
	s := Default()
	s.Nodes = []Node{{ID: "n1", Tag: "节点一", Outbound: json.RawMessage(`{"type":"socks","server":"127.0.0.1","server_port":1080}`)}}
	return s
}

func TestDecodeDefaultsAndStrictBoundary(t *testing.T) {
	s, err := Decode([]byte(`{}`))
	if err != nil || s.Inbound.MixedPort != 7890 || s.Mode != "rule" || s.Log.Level != "info" {
		t.Fatalf("安全默认值未生效: %+v %v", s, err)
	}
	for _, body := range []string{`null`, `{}`, `{"mode":"global"}`} {
		if body == `null` {
			continue
		}
		if _, err := Decode([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	bad := []string{`null`, `[]`, `{`, `{} {}`, `{"password":"SECRET"}`, `{"mode":"rule","mode":"direct"}`, `{"inbound":{"oops":true}}`, `{"log":null}`}
	for _, body := range bad {
		if _, err := Decode([]byte(body)); err == nil || strings.Contains(err.Error(), "SECRET") {
			t.Errorf("必须拒绝且不泄漏原值: %s, %v", body, err)
		}
	}
	if _, err := Decode([]byte(strings.Repeat(" ", MaxBytes+1))); err == nil {
		t.Fatal("缺少大小限制")
	}
}

func TestValidateRejectsInvalidSpec(t *testing.T) {
	tests := map[string]func(*RunSpec){
		"mode":         func(s *RunSpec) { s.Mode = "other" },
		"log":          func(s *RunSpec) { s.Log.Level = "other" },
		"port":         func(s *RunSpec) { s.Inbound.MixedPort = 0 },
		"tun stack":    func(s *RunSpec) { s.Inbound.Tun.Enabled = true; s.Inbound.Tun.Stack = "bad" },
		"tun mtu":      func(s *RunSpec) { s.Inbound.Tun.Enabled = true; s.Inbound.Tun.MTU = 500 },
		"tun exclude":  func(s *RunSpec) { s.Inbound.Tun.Enabled = true; s.Inbound.Tun.Excludes = []string{"not-cidr"} },
		"rules":        func(s *RunSpec) { s.Rules = []Rule{{ID: "r"}} },
		"dns":          func(s *RunSpec) { s.DNS.Remote = `file:///SECRET` },
		"dns mode":     func(s *RunSpec) { s.DNS.Mode = "other" },
		"dns strategy": func(s *RunSpec) { s.DNS.Strategy = "other" },
		"dns host":     func(s *RunSpec) { s.DNS.Direct = "https://resolver.example/dns-query" },
		"duplicate":    func(s *RunSpec) { s.Nodes = append(s.Nodes, s.Nodes[0]) },
		"node id":      func(s *RunSpec) { s.Nodes[0].ID = "../SECRET" },
		"node tag":     func(s *RunSpec) { s.Nodes[0].Tag = "" },
		"outbound":     func(s *RunSpec) { s.Nodes[0].Outbound = json.RawMessage(`{"type":"naive"}`) },
		"group ref": func(s *RunSpec) {
			s.Groups = []Group{{ID: "g", Name: "G", Type: "select", Members: []string{"missing"}}}
		},
		"group cycle": func(s *RunSpec) { s.Groups = []Group{{ID: "g", Name: "G", Type: "select", Members: []string{"g"}}} },
		"group type":  func(s *RunSpec) { s.Groups = []Group{{ID: "g", Name: "G", Type: "other", Members: []string{"n1"}}} },
		"group interval": func(s *RunSpec) {
			s.Groups = []Group{{ID: "g", Name: "G", Type: "urltest", Members: []string{"n1"}, TestURL: "https://example.com", Interval: "1ns"}}
		},
		"group url": func(s *RunSpec) {
			s.Groups = []Group{{ID: "g", Name: "G", Type: "urltest", Members: []string{"n1"}, TestURL: "file:///SECRET", Interval: "5m"}}
		},
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			s := sample()
			mutate(&s)
			if err := s.Validate(); err == nil || strings.Contains(err.Error(), "SECRET") {
				t.Fatalf("应拒绝且脱敏: %v", err)
			}
		})
	}
}

func TestValidateGroupGraphAndCounts(t *testing.T) {
	s := sample()
	s.Groups = []Group{{ID: "g", Name: "G", Type: "select", Members: []string{"u"}}, {ID: "u", Name: "U", Type: "urltest", Members: []string{"n1"}, TestURL: "https://example.com", Interval: "5m"}}
	if err := s.Validate(); err != nil {
		t.Fatal(err)
	}
	s.Groups[1].Members = []string{"g"}
	if err := s.Validate(); err == nil {
		t.Fatal("跨分组循环未拒绝")
	}
	s = sample()
	s.Nodes = make([]Node, MaxNodes+1)
	if err := s.Validate(); err == nil {
		t.Fatal("节点数量未限制")
	}
}

func FuzzDecode(f *testing.F) {
	f.Add([]byte(`{}`))
	f.Add([]byte(`{"nodes":[]}`))
	f.Fuzz(func(t *testing.T, data []byte) {
		s, err := Decode(data)
		if err == nil {
			if err := s.Validate(); err != nil {
				t.Fatalf("解码接受无效配置: %v", err)
			}
		}
	})
}
