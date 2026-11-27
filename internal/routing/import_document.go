package routing

import (
	"errors"
	"io"
	"strings"

	"ferry/internal/spec"
	"gopkg.in/yaml.v3"
)

func importDocument(content string) (any, error) {
	var root yaml.Node
	decoder := yaml.NewDecoder(strings.NewReader(content))
	if decoder.Decode(&root) != nil {
		return nil, errors.New("规则文档无法解析")
	}
	var extra yaml.Node
	if decoder.Decode(&extra) != io.EOF {
		return nil, errors.New("每次只能导入一个规则文档")
	}
	count := 0
	if !boundedRuleYAML(&root, 0, &count) {
		return nil, errors.New("规则文档含别名、重复字段或超过结构限制")
	}
	var value any
	if root.Decode(&value) != nil {
		return nil, errors.New("规则文档字段无效")
	}
	return value, nil
}

func boundedRuleYAML(node *yaml.Node, depth int, count *int) bool {
	*count++
	if depth > 20 || *count > 50000 || node.Kind == yaml.AliasNode || node.Anchor != "" {
		return false
	}
	if node.Kind == yaml.MappingNode {
		seen := make(map[string]bool)
		for i := 0; i < len(node.Content); i += 2 {
			key := node.Content[i]
			if key.Kind != yaml.ScalarNode || key.Tag != "!!str" || seen[key.Value] {
				return false
			}
			seen[key.Value] = true
		}
	}
	for _, child := range node.Content {
		if !boundedRuleYAML(child, depth+1, count) {
			return false
		}
	}
	return true
}

func documentRuleList(content string, singbox bool) ([]any, []string, error) {
	root, err := importDocument(content)
	if err != nil {
		return nil, nil, err
	}
	warnings := []string{}
	if object, ok := root.(map[string]any); ok {
		if route, exists := object["route"]; singbox && exists {
			if len(object) > 1 {
				warnings = append(warnings, "仅导入路由规则，文档中的其他配置未导入")
			}
			object, ok = route.(map[string]any)
			if !ok {
				return nil, nil, errors.New("路由规则文档格式无效")
			}
		}
		if len(object) > 1 {
			warnings = append(warnings, "规则列表之外的配置未导入")
		}
		root = object["rules"]
	}
	items, ok := root.([]any)
	if !ok || len(items) > spec.MaxRules {
		return nil, nil, errors.New("规则列表无效或数量超过限制")
	}
	return items, warnings, nil
}

func importStrings(value any) ([]string, bool) {
	if text, ok := value.(string); ok {
		return []string{text}, true
	}
	values, ok := value.([]any)
	if !ok {
		return nil, false
	}
	result := make([]string, 0, len(values))
	for _, value := range values {
		text, ok := value.(string)
		if !ok {
			return nil, false
		}
		result = append(result, text)
	}
	return result, true
}
