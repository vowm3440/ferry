package app

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"unicode"
)

type DeepLink struct {
	Scheme string `json:"scheme"`
	Label  string `json:"label"`
	URL    string `json:"url"`
}

type DeepLinkPreview struct {
	Scheme     string `json:"scheme"`
	Kind       string `json:"kind"` // "url" | "text"
	Name       string `json:"name"`
	Host       string `json:"host"`
	URLMasked  string `json:"urlMasked,omitempty"`
	Payload    string `json:"payload"`
	Warning    string `json:"warning,omitempty"`
}

// ShareDeepLink generates import deep links for Ferry, Clash, and sing-box clients.
func (s *Subs) ShareDeepLink(_ context.Context, payload string) ([]DeepLink, error) {
	trimmed := strings.TrimSpace(payload)
	if trimmed == "" || len(trimmed) > 4096 || strings.ContainsFunc(trimmed, unicode.IsControl) {
		return nil, errors.New("分享内容为空、过长或包含控制字符")
	}
	u, err := url.Parse(trimmed)
	if err != nil || u.Scheme == "" {
		return nil, errors.New("分享内容必须为有效链接")
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme == "http" || scheme == "https" {
		encoded := url.QueryEscape(trimmed)
		return []DeepLink{
			{Scheme: "ferry", Label: "Ferry", URL: "ferry://install-config?url=" + encoded},
			{Scheme: "clash", Label: "Clash / Mihomo", URL: "clash://install-config?url=" + encoded},
			{Scheme: "sing-box", Label: "sing-box", URL: "sing-box://import-remote-profile?url=" + encoded},
		}, nil
	}
	return []DeepLink{
		{Scheme: "ferry", Label: "Ferry", URL: "ferry://import?data=" + url.QueryEscape(trimmed)},
	}, nil
}

// ParseDeepLink inspects an incoming ferry://, clash://, or sing-box:// link
// without modifying state. The UI must display source host and ask confirmation.
func (s *System) ParseDeepLink(_ context.Context, raw string) (DeepLinkPreview, error) {
	return ParseDeepLinkURL(raw)
}

func ParseDeepLinkURL(raw string) (DeepLinkPreview, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || len(trimmed) > 8192 || strings.ContainsFunc(trimmed, unicode.IsControl) {
		return DeepLinkPreview{}, errors.New("深链内容无效或超过长度限制")
	}
	u, err := url.Parse(trimmed)
	if err != nil {
		return DeepLinkPreview{}, errors.New("深链格式无效")
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "ferry" && scheme != "clash" && scheme != "sing-box" {
		return DeepLinkPreview{}, errors.New("不支持的深链协议")
	}
	action := strings.ToLower(u.Host)
	q := u.Query()
	if action == "install-config" || action == "import-remote-profile" {
		return parseSubscriptionDeepLink(scheme, q)
	}
	if scheme == "ferry" && action == "import" {
		return parseInlineDeepLink(scheme, q)
	}
	return DeepLinkPreview{}, errors.New("不支持的深链操作类型")
}

func parseSubscriptionDeepLink(scheme string, q url.Values) (DeepLinkPreview, error) {
	target := strings.TrimSpace(q.Get("url"))
	parsed, err := subscriptionURL(target)
	if err != nil {
		return DeepLinkPreview{}, errors.New("深链中的订阅地址无效")
	}
	name := strings.TrimSpace(q.Get("name"))
	if name == "" {
		name = parsed.Hostname()
	}
	preview := DeepLinkPreview{
		Scheme:    scheme,
		Kind:      "url",
		Name:      name,
		Host:      parsed.Hostname(),
		URLMasked: parsed.Scheme + "://" + parsed.Host + "/****",
		Payload:   target,
	}
	if strings.EqualFold(parsed.Scheme, "http") {
		preview.Warning = "该深链指向未加密的 HTTP 地址，请确认来源可信"
	}
	return preview, nil
}

func parseInlineDeepLink(scheme string, q url.Values) (DeepLinkPreview, error) {
	data := strings.TrimSpace(q.Get("data"))
	if data == "" || len(data) > 4096 || strings.ContainsFunc(data, unicode.IsControl) {
		return DeepLinkPreview{}, errors.New("深链节点内容无效或过长")
	}
	host := ""
	if parsed, err := url.Parse(data); err == nil {
		host = parsed.Hostname()
	}
	return DeepLinkPreview{
		Scheme:  scheme,
		Kind:    "text",
		Name:    strings.TrimSpace(q.Get("name")),
		Host:    host,
		Payload: data,
	}, nil
}
