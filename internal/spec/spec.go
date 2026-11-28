package spec

import (
	"bytes"
	"encoding/json"
	"errors"
	"ferry/internal/spec/policy"
	"reflect"
)

func Default() RunSpec {
	return RunSpec{
		Nodes: []Node{}, Groups: []Group{}, AppGroups: []AppGroup{}, Rules: DefaultRules(),
		DNS:     DNS{Mode: "fakeip", Remote: "https://1.1.1.1/dns-query", Direct: "https://223.5.5.5/dns-query", Strategy: "prefer_ipv4"},
		Inbound: Inbound{MixedPort: 7890, Tun: Tun{Stack: "mixed", StrictRoute: true, Excludes: []string{}, MTU: 9000}},
		Mode:    "rule", Log: Log{Level: "info"},
	}
}

func Decode(data []byte) (RunSpec, error) {
	if len(data) > MaxBytes {
		return RunSpec{}, errors.New("配置超过大小限制")
	}
	if err := policy.CheckJSON(data, 16); err != nil {
		return RunSpec{}, err
	}
	if !exactFields(data, reflect.TypeFor[RunSpec]()) {
		return RunSpec{}, errors.New("配置包含未知字段或字段名称不正确")
	}
	s := Default()
	// JSON arrays reuse existing elements; a user rule must not inherit Pinned
	// or conditions from a built-in default whose optional fields are omitted.
	s.Rules = nil
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(&s); err != nil {
		return RunSpec{}, errors.New("配置包含未知字段或字段类型不正确")
	}
	if s.Rules == nil {
		s.Rules = DefaultRules()
	}
	if err := s.Validate(); err != nil {
		return RunSpec{}, err
	}
	return s, nil
}

func (s RunSpec) Validate() error {
	if err := s.validateSettings(); err != nil {
		return err
	}
	if len(s.Nodes) > MaxNodes || len(s.Groups) > MaxGroups {
		return errors.New("节点或分组数量超过限制")
	}
	ids := map[string]bool{"__direct": true}
	for _, node := range s.Nodes {
		if !validID(node.ID) || node.ID == "proxy" || ids[node.ID] || !validLabel(node.Tag) {
			return errors.New("节点标识或名称无效、重复")
		}
		ids[node.ID] = true
		if _, err := policy.SanitizeOutbound(node.Outbound); err != nil {
			return err
		}
	}
	if err := s.validateGroups(ids); err != nil {
		return err
	}
	if err := s.validateAppGroups(); err != nil {
		return err
	}
	if err := s.validateRuleSets(); err != nil {
		return err
	}
	if err := s.validateDNSRouting(); err != nil {
		return err
	}
	return s.validateRules()
}
