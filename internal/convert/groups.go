package convert

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"ferry/internal/spec"
	"fmt"
	"slices"
	"strconv"
	"time"
)

type groupDraft struct {
	name, kind, url, interval string
	members                   []string
}
type groupResolver struct {
	drafts   map[string]groupDraft
	names    map[string]string
	resolved map[string]spec.Group
	visiting map[string]bool
}

func convertGroups(items []any, format string, names map[string]string) ([]spec.Group, error) {
	if len(items) > spec.MaxGroups {
		return nil, errors.New("订阅分组超过限制")
	}
	resolver := groupResolver{drafts: map[string]groupDraft{}, names: names, resolved: map[string]spec.Group{}, visiting: map[string]bool{}}
	order := []string{}
	for _, item := range items {
		raw, ok := item.(map[string]any)
		if !ok {
			return nil, errors.New("分组格式无效")
		}
		g, err := readGroup(raw, format)
		if err != nil {
			return nil, err
		}
		if _, exists := resolver.drafts[g.name]; exists || names[g.name] != "" {
			return nil, errors.New("分组名称重复或与节点名称冲突")
		}
		resolver.drafts[g.name] = g
		order = append(order, g.name)
	}
	result := []spec.Group{}
	for _, name := range order {
		if _, err := resolver.resolve(name, 0); err != nil {
			return nil, err
		}
		result = append(result, resolver.resolved[name])
	}
	return result, nil
}

func (r *groupResolver) resolve(name string, depth int) (string, error) {
	if id := r.names[name]; id != "" {
		return id, nil
	}
	if g, ok := r.resolved[name]; ok {
		return g.ID, nil
	}
	draft, ok := r.drafts[name]
	if !ok || depth > 16 || r.visiting[name] {
		return "", errors.New("分组成员不存在、循环引用或超出层级限制")
	}
	r.visiting[name] = true
	g := spec.Group{Name: draft.name, Type: draft.kind, TestURL: draft.url, Interval: draft.interval, Members: []string{}}
	for _, member := range draft.members {
		id, err := r.resolve(member, depth+1)
		if err != nil {
			return "", err
		}
		if !slices.Contains(g.Members, id) {
			g.Members = append(g.Members, id)
		}
	}
	data, err := json.Marshal(g)
	if err != nil {
		return "", errors.New("分组无法编码")
	}
	sum := sha256.Sum256(data)
	g.ID = "g-" + hex.EncodeToString(sum[:8])
	r.visiting[name] = false
	r.resolved[name] = g
	return g.ID, nil
}

func readGroup(raw map[string]any, format string) (groupDraft, error) {
	var g groupDraft
	nameKey, membersKey := "name", "proxies"
	known := []string{"name", "type", "proxies", "url", "interval"}
	if format == "singbox" {
		nameKey, membersKey = "tag", "outbounds"
		known = []string{"tag", "type", "outbounds", "url", "interval", "default"}
	}
	for key := range raw {
		if !containsKey(known, key) {
			return g, errors.New("订阅分组含当前不支持的字段")
		}
	}
	g.name, _ = raw[nameKey].(string)
	g.kind, _ = raw["type"].(string)
	if value, exists := raw["url"]; exists {
		var ok bool
		g.url, ok = value.(string)
		if !ok {
			return g, errors.New("分组测速地址无效")
		}
	}
	if g.kind == "selector" {
		g.kind = "select"
	}
	if g.kind == "url-test" {
		g.kind = "urltest"
	}
	if g.name == "" || (g.kind != "select" && g.kind != "urltest") {
		return g, errors.New("分组名称或类型无效")
	}
	var err error
	g.members, err = groupMembers(raw[membersKey], raw["default"])
	if err != nil {
		return g, err
	}
	if value, exists := raw["interval"]; exists {
		g.interval, err = groupInterval(value, format)
		if err != nil {
			return g, err
		}
	}
	return g, nil
}

func groupMembers(value, preferred any) ([]string, error) {
	raw, ok := value.([]any)
	if !ok || len(raw) == 0 || len(raw) > spec.MaxNodes {
		return nil, errors.New("分组成员无效")
	}
	members := []string{}
	for _, item := range raw {
		name, ok := item.(string)
		if !ok || name == "" {
			return nil, errors.New("分组成员无效")
		}
		members = append(members, name)
	}
	if preferred != nil {
		selected, ok := preferred.(string)
		if !ok {
			return nil, errors.New("默认分组成员无效")
		}
		if selected != "" {
			index := slices.Index(members, selected)
			if index < 0 {
				return nil, errors.New("分组默认节点不在成员列表中")
			}
			members = append([]string{selected}, slices.Delete(slices.Clone(members), index, index+1)...)
		}
	}
	return members, nil
}

func groupInterval(value any, format string) (string, error) {
	if format == "singbox" {
		text, ok := value.(string)
		if !ok {
			return "", errors.New("分组测速间隔无效")
		}
		return text, nil
	}
	seconds, err := strconv.Atoi(fmt.Sprint(value))
	if err != nil || seconds < 0 || seconds > 86400 {
		return "", errors.New("分组测速间隔无效")
	}
	if seconds == 0 {
		return "", nil
	}
	return (time.Duration(seconds) * time.Second).String(), nil
}
