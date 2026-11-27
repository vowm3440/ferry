package convert

import (
	"net/url"
	"slices"
	"strconv"
	"strings"
)

// parseURL handles the link shapes that follow RFC 3986: scheme://user@host:port?query#name.
func parseURL(line string) (*url.URL, url.Values, string) {
	u, err := url.Parse(line)
	if err != nil || u.Hostname() == "" {
		return nil, nil, "链接无法解析"
	}
	if u.Path != "" && u.Path != "/" {
		return nil, nil, "链接含暂不支持的路径，请使用传输参数 path"
	}
	if u.Port() == "" {
		return nil, nil, "缺少端口"
	}
	if port, err := strconv.Atoi(u.Port()); err != nil || port < 1 || port > 65535 {
		return nil, nil, "端口无效"
	}
	q, reason := parseQuery(u.RawQuery)
	return u, q, reason
}

func base(kind string, u *url.URL) map[string]any {
	port, _ := strconv.Atoi(u.Port()) // range-checked by parseURL.
	return map[string]any{"type": kind, "server": u.Hostname(), "server_port": port}
}

// unsupportedParam reports the first parameter that carries a value but has no
// exact equivalent here, so the link is refused instead of imported altered.
func unsupportedParam(q url.Values, known ...string) string {
	keys := make([]string, 0, len(q))
	for key := range q {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	for _, key := range keys {
		if q.Get(key) == "" || slices.Contains(known, key) {
			continue
		}
		if plainWord(key) {
			return "含暂不支持的参数 " + key
		}
		return "含暂不支持的参数"
	}
	return ""
}

func tlsOptions(q url.Values) map[string]any {
	tls := map[string]any{"enabled": true}
	name := q.Get("sni")
	if name == "" {
		name = q.Get("peer")
	}
	if name != "" {
		tls["server_name"] = name
	}
	if truthy(q.Get("allowInsecure")) || truthy(q.Get("insecure")) {
		tls["insecure"] = true
	}
	if alpn := q.Get("alpn"); alpn != "" {
		tls["alpn"] = strings.Split(alpn, ",")
	}
	if fp := q.Get("fp"); fp != "" {
		tls["utls"] = map[string]any{"enabled": true, "fingerprint": fp}
	}
	if q.Get("security") == "reality" {
		tls["reality"] = map[string]any{"enabled": true, "public_key": normalizeRealityKey(q.Get("pbk")), "short_id": normalizeRealityShortID(q.Get("sid"))}
		if q.Get("fp") == "" {
			tls["utls"] = map[string]any{"enabled": true, "fingerprint": "chrome"}
		}
	}
	return tls
}

func truthy(value string) bool { return value == "1" || strings.EqualFold(value, "true") }

// transportOptions returns nil for plain TCP.
func transportOptions(q url.Values) (map[string]any, string) {
	if header := q.Get("headerType"); header != "" && header != "none" {
		return nil, "含暂不支持的参数 headerType"
	}
	if mode := q.Get("mode"); mode != "" && q.Get("type") != "grpc" {
		return nil, "仅 gRPC 传输支持 mode 参数"
	}
	switch q.Get("type") {
	case "grpc":
		if q.Get("host") != "" || q.Get("path") != "" {
			return nil, "gRPC 请使用 serviceName 参数"
		}
		if mode := q.Get("mode"); mode != "" && mode != "gun" {
			return nil, "暂不支持该 gRPC 模式"
		}
		return map[string]any{"type": "grpc", "service_name": q.Get("serviceName")}, ""
	case "", "tcp", "raw":
		if q.Get("host") != "" || q.Get("path") != "" || q.Get("serviceName") != "" {
			return nil, "TCP 传输不支持 host 或 path 参数"
		}
		return nil, ""
	case "ws":
		if q.Get("serviceName") != "" {
			return nil, "WebSocket 不支持 serviceName 参数"
		}
		path := q.Get("path")
		if path == "" {
			path = "/"
		}
		transport := map[string]any{"type": "ws", "path": path}
		if host := q.Get("host"); host != "" {
			transport["headers"] = map[string]any{"Host": host}
		}
		return transport, ""
	default:
		return nil, "暂不支持该传输方式"
	}
}

var streamParams = []string{"security", "sni", "peer", "type", "host", "path", "alpn", "allowInsecure", "insecure", "headerType", "fp", "serviceName", "mode"}

func parseVLESS(line string) (draft, string) {
	u, q, reason := parseURL(line)
	if reason != "" {
		return draft{}, reason
	}
	if _, present := u.User.Password(); present {
		return draft{}, "VLESS 用户信息只能包含 UUID"
	}
	if encryption := q.Get("encryption"); encryption != "" && encryption != "none" {
		return draft{}, "暂不支持该加密方式"
	}
	outbound := base("vless", u)
	outbound["uuid"] = u.User.Username()
	switch q.Get("security") {
	case "", "none":
	case "tls", "reality":
		outbound["tls"] = tlsOptions(q)
	default:
		return draft{}, "暂不支持该传输安全类型"
	}
	if reason = validateTLSParams(q, q.Get("security") == "tls" || q.Get("security") == "reality"); reason != "" {
		return draft{}, reason
	}
	if reason = applyTransport(outbound, q); reason != "" {
		return draft{}, reason
	}
	if flow := q.Get("flow"); flow != "" {
		outbound["flow"] = flow
	}
	known := append([]string{"encryption", "flow", "pbk", "sid", "spx"}, streamParams...)
	return draft{name: u.Fragment, outbound: outbound}, unsupportedParam(q, known...)
}

func parseTrojan(line string) (draft, string) {
	u, q, reason := parseURL(line)
	if reason != "" {
		return draft{}, reason
	}
	if _, present := u.User.Password(); present {
		return draft{}, "Trojan 密码中的冒号必须使用百分号编码"
	}
	password := u.User.Username()
	if password == "" {
		return draft{}, "缺少密码"
	}
	if security := q.Get("security"); security != "" && security != "tls" {
		return draft{}, "暂不支持该传输安全类型"
	}
	if reason = validateTLSParams(q, true); reason != "" {
		return draft{}, reason
	}
	outbound := base("trojan", u)
	outbound["password"] = password
	outbound["tls"] = tlsOptions(q)
	if reason = applyTransport(outbound, q); reason != "" {
		return draft{}, reason
	}
	return draft{name: u.Fragment, outbound: outbound}, unsupportedParam(q, streamParams...)
}

func applyTransport(outbound map[string]any, q url.Values) string {
	transport, reason := transportOptions(q)
	if transport != nil {
		outbound["transport"] = transport
	}
	return reason
}

func parseSocks(line string) (draft, string) {
	u, q, reason := parseURL(line)
	if reason != "" {
		return draft{}, reason
	}
	outbound := base("socks", u)
	outbound["version"] = "5"
	user, password := u.User.Username(), ""
	if u.User != nil {
		var explicit bool
		if password, explicit = u.User.Password(); !explicit {
			// Some clients pack "user:password" into base64 userinfo.
			if decoded, ok := decodeBase64(user); ok && strings.Contains(decoded, ":") {
				user, password, _ = strings.Cut(decoded, ":")
			}
		}
	}
	if user != "" {
		outbound["username"] = user
	}
	if password != "" {
		outbound["password"] = password
	}
	return draft{name: u.Fragment, outbound: outbound}, unsupportedParam(q)
}

// parseShadowsocks accepts SIP002 (userinfo@host:port) and the legacy form
// where the whole "method:password@host:port" is base64.
func parseShadowsocks(line string) (draft, string) {
	_, rest, _ := strings.Cut(line, "://")
	rest, fragment, _ := strings.Cut(rest, "#")
	name, err := url.PathUnescape(fragment)
	if err != nil {
		name = fragment
	}
	rest, rawQuery, _ := strings.Cut(rest, "?")
	query, reason := parseQuery(rawQuery)
	if reason != "" {
		return draft{}, reason
	}
	if query.Get("plugin") != "" {
		return draft{}, "暂不支持 Shadowsocks 插件"
	}
	credentials, address, ok := shadowsocksParts(strings.TrimSuffix(rest, "/"))
	if !ok {
		return draft{}, "链接无法解析"
	}
	method, password, _ := strings.Cut(credentials, ":")
	host, port, ok := splitHostPort(address)
	if !ok {
		return draft{}, "地址或端口无效"
	}
	outbound := map[string]any{"type": "shadowsocks", "server": host, "server_port": port, "method": method, "password": password}
	return draft{name: name, outbound: outbound}, unsupportedParam(query)
}

func shadowsocksParts(rest string) (credentials, address string, ok bool) {
	at := strings.LastIndex(rest, "@")
	if at < 0 {
		decoded, valid := decodeBase64(rest)
		at = strings.LastIndex(decoded, "@")
		if !valid || at < 0 {
			return "", "", false
		}
		return decoded[:at], decoded[at+1:], strings.Contains(decoded[:at], ":")
	}
	userinfo, address := rest[:at], rest[at+1:]
	if decoded, valid := decodeBase64(userinfo); valid && strings.Contains(decoded, ":") {
		return decoded, address, true
	}
	plain, err := url.PathUnescape(userinfo)
	return plain, address, err == nil && strings.Contains(plain, ":")
}
