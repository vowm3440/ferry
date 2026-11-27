package convert

import (
	"encoding/base64"
	"encoding/hex"
	"net/url"
	"strings"
	"unicode"
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
	if q.Get("security") != "reality" && (q.Get("pbk") != "" || q.Get("sid") != "" || q.Get("spx") != "") {
		return "Reality 未启用，不能设置其参数"
	}
	if spx := q.Get("spx"); len(spx) > 2048 || strings.ContainsFunc(spx, unicode.IsControl) {
		return "Reality spiderX 参数无效"
	}
	if !enabled && (sni != "" || peer != "" || q.Get("alpn") != "" || q.Get("fp") != "" || truthy(insecure) || truthy(alias)) {
		return "TLS 未启用，不能设置 TLS 参数"
	}
	return ""
}

func normalizeRealityKey(raw string) string {
	for _, enc := range []*base64.Encoding{base64.RawURLEncoding, base64.URLEncoding, base64.RawStdEncoding, base64.StdEncoding} {
		if data, err := enc.DecodeString(raw); err == nil && len(data) == 32 {
			return base64.RawURLEncoding.EncodeToString(data)
		}
	}
	return raw
}

func normalizeRealityShortID(raw string) string {
	if len(raw) <= 15 && len(raw)%2 == 1 {
		if _, err := hex.DecodeString(raw + "0"); err == nil {
			return raw + "0"
		}
	}
	return raw
}
