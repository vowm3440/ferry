package convert

import (
	"net/url"
	"strings"
)

// URL.Query silently discards malformed pairs, and Values.Get takes only the
// first duplicate. Neither is suitable for security-bearing share links.
func parseQuery(raw string) (url.Values, string) {
	query, err := url.ParseQuery(raw)
	if err != nil {
		return nil, "链接查询参数无效"
	}
	for _, values := range query {
		if len(values) != 1 {
			return nil, "链接包含重复参数"
		}
	}
	return query, ""
}

func validateTLSParams(q url.Values, enabled bool) string {
	insecure, alias := q.Get("insecure"), q.Get("allowInsecure")
	for _, value := range []string{insecure, alias} {
		switch strings.ToLower(value) {
		case "", "0", "1", "false", "true":
		default:
			return "TLS 证书校验参数无效"
		}
	}
	if insecure != "" && alias != "" && truthy(insecure) != truthy(alias) {
		return "TLS 证书校验参数互相冲突"
	}
	sni, peer := q.Get("sni"), q.Get("peer")
	if sni != "" && peer != "" && !strings.EqualFold(sni, peer) {
		return "TLS 服务器名称参数互相冲突"
	}
	if q.Get("security") != "reality" && (q.Get("pbk") != "" || q.Get("sid") != "") {
		return "Reality 未启用，不能设置其参数"
	}
	if !enabled && (sni != "" || peer != "" || q.Get("alpn") != "" || q.Get("fp") != "" || truthy(insecure) || truthy(alias)) {
		return "TLS 未启用，不能设置 TLS 参数"
	}
	return ""
}
