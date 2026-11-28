package host

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	include "ferry/internal/core/boxinclude"
	"ferry/internal/core/compile"
	"ferry/internal/spec"
	box "github.com/sagernet/sing-box"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/service"
	"net"
	"net/http"
	"time"
	F "github.com/sagernet/sing/common/format"
)

type instance struct {
	box          *box.Box
	cancel       context.CancelFunc
	address      string
	secret       string
	client       *http.Client
	groups       map[string]map[string]bool
	nodes        map[string]bool
	ruleLookup   map[string]compile.RuleMeta
	proxyTraffic *proxyTrafficTracker
	started      time.Time
}

func newInstance(options option.Options, s spec.RunSpec, logs *logBuffer) (*instance, error) {
	address, secret, err := controllerCredentials()
	if err != nil {
		return nil, err
	}
	options.Experimental = &option.ExperimentalOptions{ClashAPI: &option.ClashAPIOptions{
		ExternalController: address, Secret: secret, DefaultMode: s.Mode,
		AccessControlAllowOrigin: []string{"http://ferry.invalid"},
	}}
	ctx, cancel := context.WithCancel(service.ExtendContext(include.Context(context.Background())))
	logs.configure(s)
	core, err := box.New(box.Options{Context: ctx, Options: options})
	if err != nil {
		cancel()
		return nil, errors.New("内核配置初始化失败，请检查节点参数")
	}
	// Passing PlatformLogWriter to box.New implicitly enables sing-box's
	// cache.db. Attach after construction to preserve Ferry's own state source.
	core.LogFactory().AttachPlatformWriter(logs)
	proxyTraffic := &proxyTrafficTracker{}
	if core.Router() != nil {
		core.Router().AppendTracker(proxyTraffic)
	}
	if err := core.Start(); err != nil {
		cancel() // sing-box Start closes partial instances on failure.
		return nil, errors.New("内核启动失败，请检查代理端口是否被占用及节点参数")
	}
	return &instance{box: core, cancel: cancel, address: address, secret: secret,
		client: &http.Client{Timeout: 3 * time.Second, Transport: &http.Transport{Proxy: nil}},
		groups: selectionGroups(s), nodes: nodeSet(s), ruleLookup: buildRuleLookup(core, s),
		proxyTraffic: proxyTraffic, started: time.Now(),
	}, nil
}

func buildRuleLookup(core *box.Box, s spec.RunSpec) map[string]compile.RuleMeta {
	lookup := map[string]compile.RuleMeta{
		"final": {RuleID: spec.BuiltinDefaultRuleID, RuleName: "默认规则", Kind: "rule"},
	}
	if core == nil || core.Router() == nil {
		return lookup
	}
	compiled := core.Router().Rules()
	meta := compile.CompiledRuleMeta(s)
	for i := 0; i < len(compiled) && i < len(meta); i++ {
		if meta[i].Kind == "system" || meta[i].Kind == "resolve" {
			continue
		}
		key := F.ToString(compiled[i], " => ", compiled[i].Action())
		if _, exists := lookup[key]; !exists {
			lookup[key] = meta[i]
		}
	}
	return lookup
}

func nodeSet(s spec.RunSpec) map[string]bool {
	nodes := make(map[string]bool, len(s.Nodes))
	for _, node := range s.Nodes {
		nodes[node.ID] = true
	}
	return nodes
}

func controllerCredentials() (string, string, error) {
	var secret [32]byte
	if _, err := rand.Read(secret[:]); err != nil {
		return "", "", errors.New("无法生成内核控制凭据")
	}
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return "", "", errors.New("无法分配本机内核控制端口")
	}
	address := listener.Addr().String()
	if err := listener.Close(); err != nil {
		return "", "", errors.New("无法准备本机内核控制端口")
	}
	return address, base64.RawURLEncoding.EncodeToString(secret[:]), nil
}

func selectionGroups(s spec.RunSpec) map[string]map[string]bool {
	groups := make(map[string]map[string]bool)
	if len(s.Groups) == 0 {
		groups[compile.ProxyTag] = make(map[string]bool)
		for _, node := range s.Nodes {
			groups[compile.ProxyTag][node.ID] = true
		}
	}
	for _, group := range s.Groups {
		if group.Type != "select" {
			continue
		}
		groups[group.ID] = make(map[string]bool)
		for _, id := range group.Members {
			groups[group.ID][id] = true
		}
	}
	return groups
}

func (i *instance) close() error {
	i.cancel()
	i.client.CloseIdleConnections()
	if err := i.box.Close(); err != nil {
		return errors.New("内核关闭失败，请退出应用后重试")
	}
	return nil
}
