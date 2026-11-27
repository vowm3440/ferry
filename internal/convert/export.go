package convert

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"ferry/internal/spec"
	"fmt"
	"gopkg.in/yaml.v3"
	"strings"
	"time"
)

func Export(nodes []spec.Node, groups []spec.Group, format string) (string, error) {
	config := spec.Default()
	config.Nodes = nodes
	config.Groups = groups
	if len(nodes) == 0 || config.Validate() != nil {
		return "", errors.New("没有可导出的有效节点")
	}
	var result []byte
	var err error
	if format == "uri" || format == "base64" {
		lines := []string{}
		for _, node := range nodes {
			link, err := exportURI(node)
			if err != nil {
				return "", err
			}
			lines = append(lines, link)
		}
		result = []byte(strings.Join(lines, "\n"))
		if format == "base64" {
			result = []byte(base64.StdEncoding.EncodeToString(result))
		}
	} else if format == "singbox" || format == "clash" {
		document, buildErr := exportDocument(nodes, groups, format)
		if buildErr != nil {
			return "", buildErr
		}
		if format == "clash" {
			result, err = yaml.Marshal(document)
		} else {
			result, err = json.MarshalIndent(document, "", "  ")
		}
	} else {
		return "", errors.New("不支持此导出格式")
	}
	if err != nil || len(result) > MaxBytes {
		return "", errors.New("导出内容无法编码或超过 1 MiB")
	}
	return string(result), nil
}

func exportNames(nodes []spec.Node, groups []spec.Group) map[string]string {
	names := map[string]string{"__direct": "DIRECT"}
	used := map[string]bool{"DIRECT": true}
	add := func(id, name string) {
		if used[name] {
			name += " [" + id + "]"
		}
		for used[name] {
			name += "_"
		}
		used[name] = true
		names[id] = name
	}
	for _, node := range nodes {
		add(node.ID, node.Tag)
	}
	for _, group := range groups {
		add(group.ID, group.Name)
	}
	return names
}

func exportDocument(nodes []spec.Node, groups []spec.Group, format string) (map[string]any, error) {
	names := exportNames(nodes, groups)
	outbounds, endpoints, err := exportDocumentNodes(nodes, format, names)
	if err != nil {
		return nil, err
	}
	groupItems := exportDocumentGroups(groups, format, names)
	if format == "clash" {
		return map[string]any{"proxies": outbounds, "proxy-groups": groupItems}, nil
	}
	outbounds = append(outbounds, groupItems...)
	outbounds = append(outbounds, map[string]any{"type": "direct", "tag": "DIRECT"})
	doc := map[string]any{"outbounds": outbounds}
	if len(endpoints) > 0 {
		doc["endpoints"] = endpoints
	}
	return doc, nil
}

func exportDocumentNodes(nodes []spec.Node, format string, names map[string]string) ([]any, []any, error) {
	outbounds, endpoints := []any{}, []any{}
	for _, node := range nodes {
		value, isEndpoint, err := exportNodeEntry(node, format, names[node.ID])
		if err != nil {
			return nil, nil, err
		}
		if isEndpoint {
			endpoints = append(endpoints, value)
		} else {
			outbounds = append(outbounds, value)
		}
	}
	return outbounds, endpoints, nil
}

func exportDocumentGroups(groups []spec.Group, format string, names map[string]string) []any {
	groupItems := []any{}
	for _, group := range groups {
		members := make([]string, 0, len(group.Members))
		for _, id := range group.Members {
			members = append(members, names[id])
		}
		value := map[string]any{}
		if format == "clash" {
			kind := group.Type
			if kind == "urltest" {
				kind = "url-test"
			}
			value["name"], value["type"], value["proxies"] = names[group.ID], kind, members
		} else {
			kind := group.Type
			if kind == "select" {
				kind = "selector"
			}
			value["tag"], value["type"], value["outbounds"] = names[group.ID], kind, members
		}
		if group.TestURL != "" {
			value["url"] = group.TestURL
		}
		if group.Interval != "" {
			if format == "clash" {
				duration, _ := time.ParseDuration(group.Interval)
				value["interval"] = int(duration.Seconds())
			} else {
				value["interval"] = group.Interval
			}
		}
		groupItems = append(groupItems, value)
	}
	return groupItems
}

func exportNodeEntry(node spec.Node, format, name string) (map[string]any, bool, error) {
	var value map[string]any
	if json.Unmarshal(node.Outbound, &value) != nil {
		return nil, false, errors.New("节点无法导出")
	}
	if format == "clash" {
		mapped, err := exportClash(value)
		if err != nil {
			return nil, false, err
		}
		mapped["name"] = name
		return mapped, false, nil
	}
	if value["type"] == "wireguard" {
		return exportWireGuardSingbox(value, name), true, nil
	}
	value["tag"] = name
	return value, false, nil
}

func scalar(value any) string {
	if value == nil {
		return ""
	}
	return fmt.Sprint(value)
}
