package convert

import (
	"errors"
	"ferry/internal/spec"
	"gopkg.in/yaml.v3"
	"io"
	"regexp"
	"strings"
)

var documentKey = regexp.MustCompile(`(?m)^(proxies|proxy-groups|outbounds|endpoints|servers)\s*:`)

func structuredDocument(text string) bool {
	text = strings.TrimSpace(text)
	return strings.HasPrefix(text, "{") || strings.HasPrefix(text, "[") || documentKey.MatchString(text)
}

func parseDocument(text string) (Result, error) {
	var root yaml.Node
	decoder := yaml.NewDecoder(strings.NewReader(text))
	if decoder.Decode(&root) != nil {
		return Result{}, errors.New("订阅文档无法解析")
	}
	var extra yaml.Node
	if decoder.Decode(&extra) != io.EOF {
		return Result{}, errors.New("每次只能导入一个订阅文档")
	}
	count := 0
	if !boundedYAML(&root, 0, &count) {
		return Result{}, errors.New("订阅包含 YAML 别名或超过结构限制")
	}
	var document map[string]any
	if root.Decode(&document) != nil {
		return Result{}, errors.New("订阅文档字段无效或重复")
	}
	items, kind, err := documentItems(document)
	if err != nil {
		return Result{}, err
	}
	result, names, groups, err := documentNodes(items, kind)
	if err != nil {
		return Result{}, err
	}
	if kind == "clash" {
		if value, exists := document["proxy-groups"]; exists {
			var ok bool
			groups, ok = value.([]any)
			if !ok {
				return Result{}, errors.New("订阅分组格式无效")
			}
		}
	}
	result.Groups, err = convertGroups(groups, kind, names)
	if err != nil {
		return Result{}, err
	}
	result.Skipped = append(result.Skipped, documentWarnings(document)...)
	return result, nil
}

func documentWarnings(document map[string]any) []Skipped {
	for _, key := range []string{"rules", "route", "inbounds", "rule-providers"} {
		if _, exists := document[key]; exists {
			return []Skipped{{Reason: "此处仅导入节点和分组，文档中的规则或入站设置未应用"}}
		}
	}
	return nil
}

func boundedYAML(node *yaml.Node, depth int, count *int) bool {
	*count++
	if depth > 24 || *count > 50000 || node.Kind == yaml.AliasNode {
		return false
	}
	for _, child := range node.Content {
		if !boundedYAML(child, depth+1, count) {
			return false
		}
	}
	return true
}

func documentNodes(items []any, kind string) (Result, map[string]string, []any, error) {
	result := Result{Nodes: []spec.Node{}, Groups: []spec.Group{}, Skipped: []Skipped{}}
	names := map[string]string{"DIRECT": "__direct"}
	seen := map[string]bool{}
	groups := []any{}
	for index, item := range items {
		raw, ok := item.(map[string]any)
		if !ok {
			return result, nil, nil, errors.New("节点条目格式无效")
		}
		if kind == "singbox" && (raw["type"] == "selector" || raw["type"] == "urltest") {
			groups = append(groups, raw)
			continue
		}
		if kind == "singbox" && raw["type"] == "direct" {
			for key := range raw {
				if key != "type" && key != "tag" {
					return result, nil, nil, errors.New("直连出站含无法保留的参数，导入已取消")
				}
			}
			name, _ := raw["tag"].(string)
			names[name] = "__direct"
			continue
		}
		draft, reason := documentDraft(raw, kind)
		var node spec.Node
		if reason == "" {
			node, reason = finish(draft)
		}
		if reason != "" {
			result.Skipped = append(result.Skipped, Skipped{Line: index + 1, Reason: reason})
			continue
		}
		name := draft.name
		if name == "" {
			name = node.Tag
		}
		if previous, exists := names[name]; exists && previous != node.ID {
			return result, nil, nil, errors.New("订阅含重复或保留的节点名称")
		}
		names[name] = node.ID
		if seen[node.ID] {
			continue
		}
		seen[node.ID] = true
		result.Nodes = append(result.Nodes, node)
	}
	return result, names, groups, nil
}

func documentItems(document map[string]any) ([]any, string, error) {
	if _, hasOut := document["outbounds"]; hasOut || document["endpoints"] != nil {
		return singboxItems(document)
	}
	key, kind := "proxies", "clash"
	if _, ok := document["servers"]; ok {
		key, kind = "servers", "sip008"
	}
	items, ok := document[key].([]any)
	if !ok || len(items) > spec.MaxNodes+spec.MaxGroups+16 {
		return nil, "", errors.New("未找到节点列表或数量超过限制")
	}
	return items, kind, nil
}

func singboxItems(document map[string]any) ([]any, string, error) {
	items := []any{}
	for _, key := range []string{"outbounds", "endpoints"} {
		raw, exists := document[key]
		if !exists {
			continue
		}
		list, ok := raw.([]any)
		if !ok {
			return nil, "", errors.New("未找到节点列表或数量超过限制")
		}
		items = append(items, list...)
	}
	if len(items) > spec.MaxNodes+spec.MaxGroups+16 {
		return nil, "", errors.New("未找到节点列表或数量超过限制")
	}
	return items, "singbox", nil
}

func documentDraft(raw map[string]any, kind string) (draft, string) {
	if kind == "clash" {
		return clashDraft(raw)
	}
	if kind == "singbox" && raw["type"] == "wireguard" && raw["peers"] != nil {
		return singboxWireGuardEndpoint(raw)
	}
	outbound := map[string]any{}
	nameKey := "tag"
	if kind == "sip008" {
		nameKey = "remarks"
		outbound["type"] = "shadowsocks"
	}
	for key, value := range raw {
		if key == nameKey || (kind == "sip008" && key == "id") {
			continue
		}
		outbound[key] = value
	}
	name, _ := raw[nameKey].(string)
	return draft{name: name, outbound: outbound}, ""
}
