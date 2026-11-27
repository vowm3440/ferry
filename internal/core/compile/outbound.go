package compile

import (
	"encoding/json"
	"errors"
	"ferry/internal/core/policy"
	"ferry/internal/spec"
)

func compileOutbounds(s spec.RunSpec) ([]any, []any, error) {
	result := make([]any, 0, len(s.Nodes)+len(s.Groups)+2)
	endpoints := make([]any, 0)
	ids := make([]string, 0, len(s.Nodes))
	for _, node := range s.Nodes {
		data, err := policy.SanitizeOutbound(node.Outbound)
		if err != nil {
			return nil, nil, err
		}
		var outbound map[string]any
		if json.Unmarshal(data, &outbound) != nil {
			return nil, nil, errors.New("节点配置无效")
		}
		if outbound["type"] == "wireguard" {
			endpoints = append(endpoints, compileWireGuardEndpoint(node.ID, outbound))
		} else {
			outbound["tag"] = node.ID
			result = append(result, outbound)
		}
		ids = append(ids, node.ID)
	}
	if len(s.Groups) == 0 {
		result = append(result, map[string]any{"type": "selector", "tag": ProxyTag, "outbounds": ids, "default": ids[0]})
	}
	for _, group := range s.Groups {
		result = append(result, compileGroup(group))
	}
	return append(result, map[string]any{"type": "direct", "tag": "__direct"}), endpoints, nil
}

func compileWireGuardEndpoint(tag string, o map[string]any) map[string]any {
	peer := map[string]any{
		"address":     o["server"],
		"port":        o["server_port"],
		"public_key":  o["peer_public_key"],
		"allowed_ips": []string{"0.0.0.0/0", "::/0"},
	}
	if v, ok := o["pre_shared_key"]; ok {
		peer["pre_shared_key"] = v
	}
	if v, ok := o["reserved"]; ok {
		peer["reserved"] = v
	}
	ep := map[string]any{
		"type":        "wireguard",
		"tag":         tag,
		"address":     o["local_address"],
		"private_key": o["private_key"],
		"peers":       []any{peer},
	}
	if v, ok := o["mtu"]; ok {
		ep["mtu"] = v
	}
	return ep
}

func compileGroup(group spec.Group) map[string]any {
	result := map[string]any{"type": "selector", "tag": group.ID, "outbounds": group.Members}
	if group.Type == "select" {
		result["default"] = group.Members[0]
		return result
	}
	result["type"] = "urltest"
	if group.TestURL != "" {
		result["url"] = group.TestURL
	}
	if group.Interval != "" {
		result["interval"] = group.Interval
	}
	return result
}
