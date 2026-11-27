package app

import (
	"context"
	"encoding/base64"
	"errors"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"ferry/internal/convert"
)

func subscriptionURL(raw string) (*url.URL, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || len(raw) > 4096 || u.Hostname() == "" || u.User != nil || u.Fragment != "" || (u.Scheme != "http" && u.Scheme != "https") {
		return nil, errors.New("订阅地址须为有效的 HTTP 或 HTTPS 地址，且不能包含用户信息或片段")
	}
	return u, nil
}

func fetchSubscription(ctx context.Context, raw, ua string) ([]byte, http.Header, error) {
	u, err := subscriptionURL(raw)
	if err != nil {
		return nil, nil, err
	}
	if len(ua) > 256 || strings.ContainsAny(ua, "\r\n\x00") {
		return nil, nil, errors.New("订阅 User-Agent 无效")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, nil, errors.New("无法创建订阅请求")
	}
	request.Header.Set("User-Agent", ua)
	client := &http.Client{Timeout: 20 * time.Second, CheckRedirect: subscriptionRedirect}
	response, err := client.Do(request)
	if err != nil {
		return nil, nil, errors.New("订阅下载失败，请检查网络或地址")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, nil, errors.New("订阅服务器返回失败状态（" + strconv.Itoa(response.StatusCode) + "）")
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, convert.MaxBytes+1))
	if err != nil || len(body) > convert.MaxBytes {
		return nil, nil, errors.New("订阅读取失败或解压后的内容超过 1 MiB")
	}
	return body, response.Header.Clone(), nil
}

func subscriptionRedirect(request *http.Request, via []*http.Request) error {
	if len(via) >= 5 {
		return errors.New("订阅重定向次数过多")
	}
	if _, err := subscriptionURL(request.URL.String()); err != nil {
		return err
	}
	if len(via) > 0 && via[len(via)-1].URL.Scheme == "https" && request.URL.Scheme != "https" {
		return errors.New("拒绝将 HTTPS 订阅重定向到明文 HTTP")
	}
	request.Header.Del("Referer") // A subscription path/query often contains a token.
	return nil
}

func subscriptionMetadata(headers http.Header, raw, ua string, auto bool, interval int) Subscription {
	u, _ := subscriptionURL(raw) // checked before the request
	view := Subscription{URLMasked: u.Scheme + "://" + u.Host + "/****", UserAgent: ua, AutoUpdate: auto, IntervalHours: interval}
	if u.Scheme == "http" {
		view.Warnings = []string{"此订阅使用未加密的 HTTP，地址与节点内容可能被网络中的第三方读取"}
	}
	if hours, err := strconv.Atoi(headers.Get("profile-update-interval")); err == nil && hours >= 1 && hours <= 720 {
		view.IntervalHours = hours
	}
	values := map[string]int64{}
	for _, field := range strings.Split(headers.Get("subscription-userinfo"), ";") {
		key, value, ok := strings.Cut(strings.TrimSpace(field), "=")
		if number, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64); ok && err == nil && number >= 0 {
			values[strings.TrimSpace(key)] = number
		}
	}
	if values["upload"] <= math.MaxInt64-values["download"] {
		view.UsedBytes = values["upload"] + values["download"]
	}
	view.TotalBytes = values["total"]
	if values["expire"] <= math.MaxInt64/1000 {
		view.ExpiresAt = values["expire"] * 1000
	}
	return view
}

func profileTitle(headers http.Header) string {
	value := headers.Get("profile-title")
	if strings.HasPrefix(value, "base64:") {
		decoded, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(value, "base64:"))
		if err != nil {
			return ""
		}
		value = string(decoded)
	}
	name, err := subName(value)
	if err != nil {
		return ""
	}
	return name
}
