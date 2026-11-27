package convert

import (
	"encoding/json"
	"ferry/internal/spec/policy"
	"net/url"
	"strconv"
	"strings"
)

func vmessValues(line string) (map[string]string, string) {
	_, payload, _ := strings.Cut(line, "://")
	decoded, ok := decodeBase64(payload)
	if !ok || policy.CheckJSON([]byte(decoded), 8) != nil {
		return nil, "vmess 配置无法解析"
	}
	var raw map[string]json.RawMessage
	if json.Unmarshal([]byte(decoded), &raw) != nil {
		return nil, "vmess 配置无法解析"
	}
	known := strings.Fields("v ps add port id aid scy net type host path tls sni alpn fp allowInsecure insecure")
	values := map[string]string{}
	for key, value := range raw {
		if !containsKey(known, key) {
			return nil, "vmess 含暂不支持的参数"
		}
		var text string
		if json.Unmarshal(value, &text) != nil {
			var number int64
			if json.Unmarshal(value, &number) != nil {
				return nil, "vmess 字段类型无效"
			}
			text = strconv.FormatInt(number, 10)
		}
		values[key] = text
	}
	return values, ""
}

func parseVMess(line string) (draft, string) {
	values, reason := vmessValues(line)
	if reason != "" {
		return draft{}, reason
	}
	if values["v"] != "" && values["v"] != "2" {
		return draft{}, "vmess 分享版本不支持"
	}
	port, err := strconv.Atoi(values["port"])
	if err != nil || port < 1 || port > 65535 {
		return draft{}, "vmess 端口无效"
	}
	aid := 0
	if values["aid"] != "" {
		aid, err = strconv.Atoi(values["aid"])
		if err != nil {
			return draft{}, "vmess alterId 无效"
		}
	}
	security := values["scy"]
	if security == "" {
		security = "auto"
	}
	o := map[string]any{"type": "vmess", "server": values["add"], "server_port": port, "uuid": values["id"], "security": security, "alter_id": aid}
	q := url.Values{}
	for _, key := range []string{"sni", "alpn", "fp", "allowInsecure", "insecure", "host", "path"} {
		q.Set(key, values[key])
	}
	q.Set("type", values["net"])
	q.Set("headerType", values["type"])
	if values["net"] == "grpc" {
		q.Set("serviceName", values["path"])
		q.Del("path")
	}
	if values["tls"] != "" && values["tls"] != "none" && values["tls"] != "tls" {
		return draft{}, "vmess TLS 参数不支持"
	}
	if reason := validateTLSParams(q, values["tls"] == "tls"); reason != "" {
		return draft{}, reason
	}
	if values["tls"] == "tls" {
		o["tls"] = tlsOptions(q)
	}
	if reason := applyTransport(o, q); reason != "" {
		return draft{}, reason
	}
	return draft{name: values["ps"], outbound: o}, ""
}

func containsKey(keys []string, key string) bool {
	for _, known := range keys {
		if key == known {
			return true
		}
	}
	return false
}
