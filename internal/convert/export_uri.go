package convert

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"ferry/internal/spec"
	"net"
	"net/url"
	"strings"
)

func exportURI(node spec.Node) (string, error) {
	var o map[string]any
	if json.Unmarshal(node.Outbound, &o) != nil {
		return "", errors.New("节点无法编码")
	}
	kind := scalar(o["type"])
	if kind == "http" || o["network"] != nil {
		return "", errors.New("此节点的参数无法完整表示为 URI，请使用 sing-box JSON")
	}
	u := &url.URL{Scheme: kind, Host: net.JoinHostPort(scalar(o["server"]), scalar(o["server_port"])), Fragment: node.Tag}
	q := url.Values{}
	if err := exportTLS(q, o, kind); err != nil {
		return "", err
	}
	if err := exportTransport(q, o); err != nil {
		return "", err
	}
	if kind == "vmess" {
		return exportVMess(node.Tag, o, q)
	}
	if err := setURIUser(u, q, o, kind); err != nil {
		return "", err
	}
	for key, query := range map[string]string{"up_mbps": "upmbps", "down_mbps": "downmbps", "congestion_control": "congestion_control", "udp_relay_mode": "udp_relay_mode", "zero_rtt_handshake": "zero_rtt_handshake"} {
		if value, ok := o[key]; ok {
			q.Set(query, scalar(value))
		}
	}
	if obfs, ok := o["obfs"].(map[string]any); ok {
		q.Set("obfs", scalar(obfs["type"]))
		q.Set("obfs-password", scalar(obfs["password"]))
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func setURIUser(u *url.URL, q url.Values, o map[string]any, kind string) error {
	switch kind {
	case "vless":
		u.User = url.User(scalar(o["uuid"]))
		q.Set("encryption", "none")
		if o["flow"] != nil {
			q.Set("flow", scalar(o["flow"]))
		}
	case "trojan", "hysteria2", "anytls":
		u.User = url.User(scalar(o["password"]))
	case "tuic":
		u.User = url.UserPassword(scalar(o["uuid"]), scalar(o["password"]))
	case "shadowsocks":
		u.Scheme = "ss"
		u.User = url.User(base64.RawURLEncoding.EncodeToString([]byte(scalar(o["method"]) + ":" + scalar(o["password"]))))
	case "socks":
		u.Scheme = "socks5"
		if o["version"] != nil && o["version"] != "5" {
			return errors.New("URI 导出仅支持 SOCKS5")
		}
		if o["username"] != nil {
			u.User = url.UserPassword(scalar(o["username"]), scalar(o["password"]))
		}
	case "wireguard":
		return exportWireGuardURI(u, q, o)
	default:
		return errors.New("此协议尚不支持 URI 导出")
	}
	return nil
}

func exportTLS(q url.Values, o map[string]any, kind string) error {
	tls, ok := o["tls"].(map[string]any)
	if !ok || tls["enabled"] != true {
		return nil
	}
	q.Set("security", "tls")
	if tls["server_name"] != nil {
		q.Set("sni", scalar(tls["server_name"]))
	}
	if tls["insecure"] != nil {
		q.Set("insecure", scalar(tls["insecure"]))
	}
	if alpn, ok := tls["alpn"].([]any); ok {
		items := []string{}
		for _, value := range alpn {
			items = append(items, scalar(value))
		}
		q.Set("alpn", strings.Join(items, ","))
	}
	if utls, ok := tls["utls"].(map[string]any); ok {
		q.Set("fp", scalar(utls["fingerprint"]))
	}
	if reality, ok := tls["reality"].(map[string]any); ok {
		q.Set("security", "reality")
		q.Set("pbk", scalar(reality["public_key"]))
		q.Set("sid", scalar(reality["short_id"]))
	}
	if tls["disable_sni"] != nil {
		if kind != "tuic" {
			return errors.New("此 TLS 参数无法表示为 URI，请使用 JSON 导出")
		}
		q.Set("disable_sni", scalar(tls["disable_sni"]))
	}
	return nil
}

func exportTransport(q url.Values, o map[string]any) error {
	t, ok := o["transport"].(map[string]any)
	if !ok {
		return nil
	}
	q.Set("type", scalar(t["type"]))
	switch t["type"] {
	case "grpc":
		q.Set("serviceName", scalar(t["service_name"]))
	case "ws":
		q.Set("path", scalar(t["path"]))
		if headers, ok := t["headers"].(map[string]any); ok {
			for key, value := range headers {
				if !strings.EqualFold(key, "Host") {
					return errors.New("自定义 HTTP 请求头无法完整表示为 URI")
				}
				q.Set("host", scalar(value))
			}
		}
	default:
		return errors.New("此传输参数无法表示为 URI，请使用 JSON 导出")
	}
	return nil
}

func exportVMess(name string, o map[string]any, q url.Values) (string, error) {
	v := map[string]any{"v": "2", "ps": name, "add": o["server"], "port": scalar(o["server_port"]), "id": o["uuid"], "aid": "0", "scy": "auto", "net": "tcp", "type": "none", "host": q.Get("host"), "path": q.Get("path"), "tls": "", "sni": q.Get("sni"), "alpn": q.Get("alpn")}
	if o["alter_id"] != nil {
		v["aid"] = scalar(o["alter_id"])
	}
	if o["security"] != nil {
		v["scy"] = o["security"]
	}
	if q.Get("type") != "" {
		v["net"] = q.Get("type")
	}
	if q.Get("type") == "grpc" {
		v["path"] = q.Get("serviceName")
	}
	if q.Get("security") != "" {
		if q.Get("security") != "tls" {
			return "", errors.New("VMess URI 不支持此 TLS 选项")
		}
		v["tls"] = "tls"
	}
	for _, key := range []string{"insecure", "fp"} {
		if q.Get(key) != "" {
			v[key] = q.Get(key)
		}
	}
	data, err := json.Marshal(v)
	if err != nil {
		return "", errors.New("VMess 无法编码")
	}
	return "vmess://" + base64.StdEncoding.EncodeToString(data), nil
}
