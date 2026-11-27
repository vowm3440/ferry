package routing

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"ferry/internal/spec"
)

type ImportPreview struct {
	Rules    []spec.Rule `json:"rules"`
	Warnings []string    `json:"warnings"`
}

type importedRule struct {
	rule   spec.Rule
	reason string
	notice string
}

// Import produces a bounded, side-effect-free preview. Unsupported semantics
// skip the entire source rule, with a warning that never echoes raw input.
func Import(kind, content string, s spec.RunSpec) (ImportPreview, error) {
	preview := ImportPreview{Rules: []spec.Rule{}, Warnings: []string{}}
	if len(content) > spec.MaxBytes {
		return preview, errors.New("规则内容超过 1 MiB 限制")
	}
	content = strings.TrimSpace(strings.TrimPrefix(content, "\ufeff"))
	if content == "" {
		return preview, errors.New("规则内容为空")
	}
	var items []importedRule
	var err error
	switch kind {
	case "clash":
		items, preview.Warnings, err = importClash(content, s)
	case "singbox":
		items, preview.Warnings, err = importSingbox(content, s)
	case "ppx":
		items, preview.Warnings, err = importPPX(content, s)
	default:
		return preview, errors.New("规则导入格式不受支持")
	}
	if err != nil {
		return ImportPreview{}, err
	}
	if len(items) > spec.MaxRules {
		return ImportPreview{}, errors.New("规则数量超过限制")
	}
	for i, item := range items {
		r := item.rule
		if r.Name == "" {
			r.Name = fmt.Sprintf("导入规则 %d", i+1)
		}
		data, _ := json.Marshal(r)
		r.ID = fmt.Sprintf("import-%s-%d-%x", kind, i+1, sha256.Sum256(data))[:60]
		if item.reason == "" && s.ValidateRule(r) != nil {
			item.reason = "条件或目标无法完整转换"
		}
		if item.reason != "" {
			preview.Warnings = append(preview.Warnings, fmt.Sprintf("第 %d 条规则已跳过：%s", i+1, item.reason))
			continue
		}
		if item.notice != "" {
			preview.Warnings = append(preview.Warnings, fmt.Sprintf("第 %d 条规则：%s", i+1, item.notice))
		}
		preview.Rules = append(preview.Rules, r)
	}
	return preview, nil
}

func importBaseRule() spec.Rule {
	return spec.Rule{Enabled: true, Apps: []string{}, Hosts: []string{}, Ports: []string{}, Network: "any"}
}

func importAction(s spec.RunSpec, value string, r *spec.Rule) bool {
	switch value {
	case "DIRECT", "__direct":
		r.Action = "direct"
		return true
	case "REJECT":
		r.Action = "block"
		return true
	}
	ids := make(map[string]bool)
	for _, n := range s.Nodes {
		if n.ID == value || n.Tag == value {
			ids[n.ID] = true
		}
	}
	for _, g := range s.Groups {
		if g.ID == value || g.Name == value {
			ids[g.ID] = true
		}
	}
	if len(ids) != 1 {
		return false
	}
	for id := range ids {
		r.Action, r.TargetID = "proxy", id
	}
	return true
}

const catchAllNotice = "已转为普通兜底规则，应用时位于内建默认规则之前，不会替换固定规则"

func importLiteralHost(value string) bool {
	return value != "" && !strings.ContainsAny(value, "*?") && len(ipPrefixes(value)) == 0
}
