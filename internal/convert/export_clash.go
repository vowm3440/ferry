package convert

import "errors"

func exportClash(o map[string]any) (map[string]any, error) {
	kind := scalar(o["type"])
	if kind == "shadowsocks" {
		kind = "ss"
	}
	if kind == "socks" {
		if version, ok := o["version"]; ok && version != "5" {
			return nil, errors.New("SOCKS4 节点无法无损导出为 Clash，请使用 sing-box JSON")
		}
		kind = "socks5"
	}
	r := map[string]any{"type": kind, "server": o["server"], "port": o["server_port"]}
	for _, key := range []string{"uuid", "username", "password"} {
		if value, ok := o[key]; ok {
			r[key] = value
		}
	}
	for key, field := range map[string]string{"method": "cipher", "security": "cipher", "alter_id": "alterId", "up_mbps": "up", "down_mbps": "down", "congestion_control": "congestion-controller", "udp_relay_mode": "udp-relay-mode"} {
		if value, ok := o[key]; ok {
			r[field] = value
		}
	}
	if value, ok := o["network"]; ok {
		if value != "tcp" {
			return nil, errors.New("仅 UDP 的节点请使用 sing-box JSON 导出")
		}
		r["udp"] = false
	}
	if o["flow"] != nil || o["zero_rtt_handshake"] != nil {
		return nil, errors.New("此节点参数尚不支持 Clash 导出，请使用 sing-box JSON")
	}
	if err := exportClashTLS(r, o); err != nil {
		return nil, err
	}
	if err := exportClashTransport(r, o); err != nil {
		return nil, err
	}
	if obfs, ok := o["obfs"].(map[string]any); ok {
		r["obfs"], r["obfs-password"] = obfs["type"], obfs["password"]
	}
	if kind == "wireguard" {
		if err := exportWireGuardClash(r, o); err != nil {
			return nil, err
		}
	}
	return r, nil
}

func exportClashTLS(r, o map[string]any) error {
	if tls, ok := o["tls"].(map[string]any); ok {
		r["tls"] = tls["enabled"]
		for key, field := range map[string]string{"server_name": "servername", "insecure": "skip-cert-verify"} {
			if value, ok := tls[key]; ok {
				r[field] = value
			}
		}
		if tls["alpn"] != nil || tls["disable_sni"] != nil {
			return errors.New("此 TLS 参数尚不支持 Clash 导出，请使用 sing-box JSON")
		}
		if utls, ok := tls["utls"].(map[string]any); ok {
			r["client-fingerprint"] = utls["fingerprint"]
		}
		if reality, ok := tls["reality"].(map[string]any); ok {
			value := map[string]any{"public-key": reality["public_key"]}
			if short, ok := reality["short_id"]; ok {
				value["short-id"] = short
			}
			r["reality-opts"] = value
		}
	}
	return nil
}

func exportClashTransport(r, o map[string]any) error {
	if transport, ok := o["transport"].(map[string]any); ok {
		r["network"] = transport["type"]
		switch transport["type"] {
		case "ws":
			opts := map[string]any{}
			for _, key := range []string{"path", "headers"} {
				if value, ok := transport[key]; ok {
					opts[key] = value
				}
			}
			r["ws-opts"] = opts
		case "grpc":
			r["grpc-opts"] = map[string]any{"grpc-service-name": transport["service_name"]}
		default:
			return errors.New("此传输方式尚不支持 Clash 导出")
		}
	}
	return nil
}
