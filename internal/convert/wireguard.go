package convert

import (
	"errors"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
)

func parseWireGuard(line string) (draft, string) {
	u, q, reason := parseURL(line)
	if reason != "" {
		return draft{}, reason
	}
	if _, present := u.User.Password(); present {
		return draft{}, "WireGuard 私钥格式无效"
	}
	privKey := u.User.Username()
	if privKey == "" {
		return draft{}, "WireGuard 缺少私钥"
	}
	o := base("wireguard", u)
	o["private_key"] = privKey
	o["peer_public_key"] = q.Get("publickey")
	addrs, ok := parseWGAddressList(q.Get("address"))
	if !ok {
		return draft{}, "WireGuard 本地地址无效"
	}
	o["local_address"] = addrs
	if psk := q.Get("presharedkey"); psk != "" {
		o["pre_shared_key"] = psk
	}
	if reason := applyWGExtras(o, q); reason != "" {
		return draft{}, reason
	}
	known := []string{"publickey", "address", "presharedkey", "reserved", "mtu"}
	return draft{name: u.Fragment, outbound: o}, unsupportedParam(q, known...)
}

func applyWGExtras(o map[string]any, q url.Values) string {
	if raw := q.Get("mtu"); raw != "" {
		mtu, err := strconv.Atoi(raw)
		if err != nil {
			return "WireGuard MTU 无效"
		}
		o["mtu"] = mtu
	}
	if raw := q.Get("reserved"); raw != "" {
		parts := strings.Split(raw, ",")
		if len(parts) != 3 {
			return "WireGuard reserved 必须为 3 个字节"
		}
		res := make([]int, 3)
		for i, part := range parts {
			v, err := strconv.Atoi(strings.TrimSpace(part))
			if err != nil || v < 0 || v > 255 {
				return "WireGuard reserved 数值无效"
			}
			res[i] = v
		}
		o["reserved"] = res
	}
	return ""
}

func parseWGAddressList(raw string) ([]string, bool) {
	var result []string
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if addr, err := netip.ParseAddr(part); err == nil {
			result = append(result, netip.PrefixFrom(addr, addr.BitLen()).String())
			continue
		}
		prefix, err := netip.ParsePrefix(part)
		if err != nil {
			return nil, false
		}
		result = append(result, prefix.String())
	}
	return result, len(result) > 0
}

func clashWireGuard(o, raw map[string]any) string {
	addrs := []string{}
	for _, key := range []string{"ip", "ipv6"} {
		if v, ok := raw[key]; ok {
			text, valid := v.(string)
			if !valid {
				return "WireGuard 本地地址无效"
			}
			parsed, ok := parseWGAddressList(text)
			if !ok {
				return "WireGuard 本地地址无效"
			}
			addrs = append(addrs, parsed...)
		}
	}
	if len(addrs) == 0 {
		return "WireGuard 缺少本地地址"
	}
	o["local_address"] = addrs
	for key, field := range map[string]string{
		"private-key":    "private_key",
		"public-key":     "peer_public_key",
		"pre-shared-key": "pre_shared_key",
		"reserved":       "reserved",
		"mtu":            "mtu",
	} {
		if v, exists := raw[key]; exists {
			o[field] = v
		}
	}
	return ""
}

func singboxWireGuardEndpoint(raw map[string]any) (draft, string) {
	known := []string{"type", "tag", "address", "private_key", "mtu", "peers"}
	for key := range raw {
		if !containsKey(known, key) {
			return draft{}, "WireGuard 端点含暂不支持的字段"
		}
	}
	peers, ok := raw["peers"].([]any)
	if !ok || len(peers) != 1 {
		return draft{}, "WireGuard 端点必须包含单个对端配置"
	}
	peer, ok := peers[0].(map[string]any)
	if !ok {
		return draft{}, "WireGuard 对端配置无效"
	}
	return buildWireGuardEndpointDraft(raw, peer)
}

func buildWireGuardEndpointDraft(raw, peer map[string]any) (draft, string) {
	peerKnown := []string{"address", "port", "public_key", "pre_shared_key", "allowed_ips", "reserved"}
	for key := range peer {
		if !containsKey(peerKnown, key) {
			return draft{}, "WireGuard 对端含暂不支持的字段"
		}
	}
	o := map[string]any{
		"type":            "wireguard",
		"server":          peer["address"],
		"server_port":     peer["port"],
		"local_address":   raw["address"],
		"private_key":     raw["private_key"],
		"peer_public_key": peer["public_key"],
	}
	if v, ok := peer["pre_shared_key"]; ok {
		o["pre_shared_key"] = v
	}
	if v, ok := peer["reserved"]; ok {
		o["reserved"] = v
	}
	if v, ok := raw["mtu"]; ok {
		o["mtu"] = v
	}
	name, _ := raw["tag"].(string)
	return draft{name: name, outbound: o}, ""
}

func exportWireGuardURI(u *url.URL, q url.Values, o map[string]any) error {
	u.User = url.User(scalar(o["private_key"]))
	q.Set("publickey", scalar(o["peer_public_key"]))
	addrs, ok := o["local_address"].([]any)
	if !ok || len(addrs) == 0 {
		return errors.New("WireGuard 缺少本地地址")
	}
	items := make([]string, 0, len(addrs))
	for _, a := range addrs {
		items = append(items, scalar(a))
	}
	q.Set("address", strings.Join(items, ","))
	if psk, ok := o["pre_shared_key"]; ok {
		q.Set("presharedkey", scalar(psk))
	}
	if mtu, ok := o["mtu"]; ok {
		q.Set("mtu", scalar(mtu))
	}
	if res, ok := o["reserved"].([]any); ok && len(res) == 3 {
		q.Set("reserved", scalar(res[0])+","+scalar(res[1])+","+scalar(res[2]))
	}
	return nil
}

func exportWireGuardClash(r, o map[string]any) error {
	r["private-key"] = o["private_key"]
	r["public-key"] = o["peer_public_key"]
	if v, ok := o["pre_shared_key"]; ok {
		r["pre-shared-key"] = v
	}
	if v, ok := o["mtu"]; ok {
		r["mtu"] = v
	}
	if v, ok := o["reserved"]; ok {
		r["reserved"] = v
	}
	addrs, _ := o["local_address"].([]any)
	for _, item := range addrs {
		prefix, err := netip.ParsePrefix(scalar(item))
		if err != nil {
			return errors.New("WireGuard 地址无效")
		}
		key := "ip"
		if prefix.Addr().Is6() {
			key = "ipv6"
		}
		if _, exists := r[key]; exists {
			return errors.New("Clash 仅支持单个 IPv4/IPv6 地址，请使用 sing-box JSON")
		}
		r[key] = prefix.String()
	}
	return nil
}

func exportWireGuardSingbox(o map[string]any, name string) map[string]any {
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
		"tag":         name,
		"address":     o["local_address"],
		"private_key": o["private_key"],
		"peers":       []any{peer},
	}
	if v, ok := o["mtu"]; ok {
		ep["mtu"] = v
	}
	return ep
}
