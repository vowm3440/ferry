// Package compile maps a validated RunSpec to core options without side effects.
package compile

import (
	"context"
	"encoding/json"
	"errors"
	include "ferry/internal/core/boxinclude"
	"ferry/internal/spec"
	"github.com/sagernet/sing-box/option"
	corejson "github.com/sagernet/sing/common/json"
)

const ProxyTag = "proxy"

func Compile(s spec.RunSpec) (option.Options, error) {
	if err := s.Validate(); err != nil {
		return option.Options{}, err
	}
	if len(s.Nodes) == 0 {
		return option.Options{}, errors.New("请先配置至少一个代理节点")
	}
	outbounds, endpoints, err := compileOutbounds(s)
	if err != nil {
		return option.Options{}, err
	}
	route, err := compileRoute(s)
	if err != nil {
		return option.Options{}, err
	}
	inbounds := compileInbounds(s)
	config := map[string]any{
		"log":       map[string]any{"level": s.Log.Level},
		"inbounds":  inbounds,
		"outbounds": outbounds, "route": route, "dns": compileDNS(s),
	}
	if len(endpoints) > 0 {
		config["endpoints"] = endpoints
	}
	data, err := json.Marshal(config)
	if err != nil {
		return option.Options{}, errors.New("无法生成内核配置")
	}
	var options option.Options
	if err := corejson.UnmarshalContext(include.Context(context.Background()), data, &options); err != nil {
		return option.Options{}, errors.New("当前内核不支持此节点配置")
	}
	return options, nil
}

func compileInbounds(s spec.RunSpec) []any {
	listen := "127.0.0.1"
	if s.Inbound.AllowLAN {
		listen = "0.0.0.0"
	}
	inbounds := []any{map[string]any{"type": "mixed", "tag": "__mixed", "listen": listen, "listen_port": s.Inbound.MixedPort}}
	if s.Inbound.Tun.Enabled {
		inbounds = append(inbounds, compileTunInbound(s.Inbound.Tun))
	}
	return inbounds
}

func compileTunInbound(t spec.Tun) map[string]any {
	stack := t.Stack
	if stack == "" {
		stack = "mixed"
	}
	mtu := t.MTU
	if mtu == 0 {
		mtu = 9000
	}
	tun := map[string]any{
		"type":           "tun",
		"tag":            "__tun",
		"interface_name": "FerryTun",
		"address":        []string{"172.19.0.1/30", "fdfe:dcba:9876::1/126"},
		"mtu":            mtu,
		"auto_route":     true,
		"strict_route":   t.StrictRoute,
		"stack":          stack,
	}
	if len(t.Excludes) > 0 {
		tun["route_exclude_address"] = t.Excludes
	}
	return tun
}

func defaultProxy(s spec.RunSpec) string {
	if len(s.Groups) > 0 {
		return s.Groups[0].ID
	}
	return ProxyTag
}
