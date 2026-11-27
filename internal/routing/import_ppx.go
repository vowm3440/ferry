package routing

import (
	"encoding/xml"
	"errors"
	"io"
	"strings"

	"ferry/internal/spec"
)

type ppxElement struct {
	name     string
	attrs    map[string]string
	text     strings.Builder
	children []*ppxElement
}

func importPPX(content string, s spec.RunSpec) ([]importedRule, []string, error) {
	root, err := parsePPX(content)
	if err != nil {
		return nil, nil, err
	}
	warnings := []string{}
	list := root
	if root.name == "ProxifierProfile" {
		list = nil
		for _, child := range root.children {
			if child.name == "RuleList" {
				if list != nil {
					return nil, nil, errors.New("PPX 含重复规则列表")
				}
				list = child
			}
		}
		if len(root.children) > 1 {
			warnings = append(warnings, "仅导入 PPX 规则列表，代理服务器和其他配置未导入")
		}
	}
	if list == nil || list.name != "RuleList" || len(list.children) > spec.MaxRules {
		return nil, nil, errors.New("PPX 规则列表无效或数量超过限制")
	}
	if len(list.attrs) != 0 || strings.TrimSpace(list.text.String()) != "" {
		return nil, nil, errors.New("PPX 规则列表含不支持的属性或内容")
	}
	result := make([]importedRule, 0, len(list.children))
	for _, child := range list.children {
		result = append(result, ppxRule(child, s))
	}
	return result, warnings, nil
}

func parsePPX(content string) (*ppxElement, error) {
	decoder := xml.NewDecoder(strings.NewReader(content))
	var root *ppxElement
	stack := []*ppxElement{}
	for count := 0; ; count++ {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil || count > 50000 {
			return nil, errors.New("PPX 文档无效或超过结构限制")
		}
		switch token := token.(type) {
		case xml.Directive:
			return nil, errors.New("PPX 不支持 DTD 或外部实体")
		case xml.ProcInst:
			if token.Target != "xml" || root != nil {
				return nil, errors.New("PPX 不支持处理指令")
			}
		case xml.StartElement:
			if len(stack) >= 20 || token.Name.Space != "" {
				return nil, errors.New("PPX 文档超过层级限制或含命名空间")
			}
			node, valid := newPPXElement(token)
			if !valid {
				return nil, errors.New("PPX 属性无效或重复")
			}
			if len(stack) == 0 {
				if root != nil {
					return nil, errors.New("PPX 含多个根节点")
				}
				root = node
			} else {
				stack[len(stack)-1].children = append(stack[len(stack)-1].children, node)
			}
			stack = append(stack, node)
		case xml.EndElement:
			stack = stack[:len(stack)-1]
		case xml.CharData:
			if err := ppxText(stack, token); err != nil {
				return nil, err
			}
		}
	}
	if root == nil || len(stack) != 0 {
		return nil, errors.New("PPX 文档不完整")
	}
	return root, nil
}

func ppxText(stack []*ppxElement, text []byte) error {
	if len(stack) > 0 {
		stack[len(stack)-1].text.Write(text)
		return nil
	}
	if strings.TrimSpace(string(text)) != "" {
		return errors.New("PPX 根节点之外含内容")
	}
	return nil
}

func newPPXElement(token xml.StartElement) (*ppxElement, bool) {
	node := &ppxElement{name: token.Name.Local, attrs: map[string]string{}}
	for _, attr := range token.Attr {
		if _, exists := node.attrs[attr.Name.Local]; exists || attr.Name.Space != "" {
			return nil, false
		}
		node.attrs[attr.Name.Local] = attr.Value
	}
	return node, true
}

func ppxRule(node *ppxElement, s spec.RunSpec) importedRule {
	if node.name != "Rule" || strings.TrimSpace(node.text.String()) != "" {
		return importedRule{reason: "PPX 条目类型不受支持"}
	}
	r := importBaseRule()
	for key, value := range node.attrs {
		if key != "enabled" || value != "true" && value != "false" {
			return importedRule{reason: "PPX 规则属性不受支持"}
		}
		r.Enabled = value == "true"
	}
	seen := make(map[string]bool)
	for _, child := range node.children {
		if seen[child.name] || len(child.children) > 0 {
			return importedRule{reason: "PPX 规则字段重复或结构不受支持"}
		}
		seen[child.name] = true
		if !ppxField(child, s, &r) {
			return importedRule{reason: "PPX 字段或代理引用无法完整转换"}
		}
	}
	if !seen["Action"] {
		return importedRule{reason: "PPX 规则缺少动作"}
	}
	result := importedRule{rule: r}
	if r.Enabled && len(r.Apps)+len(r.Hosts)+len(r.Ports) == 0 {
		result.notice = catchAllNotice
	}
	return result
}

func ppxField(node *ppxElement, s spec.RunSpec, r *spec.Rule) bool {
	value := strings.TrimSpace(node.text.String())
	if node.name == "Action" {
		if len(node.attrs) != 1 {
			return false
		}
		switch node.attrs["type"] {
		case "Direct":
			if value != "" {
				return false
			}
			r.Action = "direct"
		case "Block":
			if value != "" {
				return false
			}
			r.Action = "block"
		case "Proxy":
			if !importAction(s, value, r) || r.Action != "proxy" {
				return false
			}
		default:
			return false
		}
		return true
	}
	if len(node.attrs) > 0 {
		return false
	}
	switch node.name {
	case "Name":
		r.Name = value
	case "Comment":
		// Informational note in Proxifier profiles; no routing effect.
	case "Applications":
		r.Apps = ppxValues(value)
	case "Targets":
		r.Hosts = ppxValues(value)
	case "Ports":
		r.Ports = ppxValues(value)
	default:
		return false
	}
	return true
}

func ppxValues(value string) []string {
	result := []string{}
	for _, part := range strings.Split(value, ";") {
		part = strings.TrimSpace(part)
		if part == "*" || strings.EqualFold(part, "any") {
			return []string{}
		}
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}
