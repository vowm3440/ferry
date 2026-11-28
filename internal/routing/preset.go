package routing

import (
	"errors"
	"slices"
	"strings"

	"ferry/internal/spec"
)

const communityBase = "https://raw.githubusercontent.com/MetaCubeX/meta-rules-dat/sing/geo/geosite/"

// CommunityRuleSets returns curated community SRS definitions ready for one-click add.
func CommunityRuleSets() []spec.RuleSet {
	return []spec.RuleSet{
		{ID: "openai", Name: "OpenAI / ChatGPT", URL: communityBase + "openai.srs", AutoUpdate: true, UpdateHours: 24},
		{ID: "anthropic", Name: "Claude / Anthropic", URL: communityBase + "anthropic.srs", AutoUpdate: true, UpdateHours: 24},
		{ID: "apple-cn", Name: "Apple 国内直连", URL: communityBase + "apple-cn.srs", AutoUpdate: true, UpdateHours: 24},
		{ID: "steam-cn", Name: "Steam 国内直连", URL: communityBase + "steam@cn.srs", AutoUpdate: true, UpdateHours: 24},
		{ID: "ads", Name: "广告与跟踪拦截", URL: communityBase + "category-ads-all.srs", AutoUpdate: true, UpdateHours: 24},
		{ID: "github", Name: "GitHub", URL: communityBase + "github.srs", AutoUpdate: true, UpdateHours: 24},
		{ID: "google", Name: "Google", URL: communityBase + "google.srs", AutoUpdate: true, UpdateHours: 24},
	}
}

// LazyPresetOptions configures the one-click lazy split-routing preset.
type LazyPresetOptions struct {
	ReplaceExisting bool     `json:"replaceExisting"`
	EnableAds       bool     `json:"enableAds"`
	EnableAI        bool     `json:"enableAi"`
	EnableAppleSteam bool    `json:"enableAppleSteam"`
	EnableCNSplit   bool     `json:"enableCnSplit"`
	EnableSplitDNS  bool     `json:"enableSplitDns"`
	IntranetDNS     string   `json:"intranetDns"`
	IntranetDomains []string `json:"intranetDomains"`
}

func DefaultLazyPresetOptions() LazyPresetOptions {
	return LazyPresetOptions{
		ReplaceExisting:  true,
		EnableAds:        true,
		EnableAI:         true,
		EnableAppleSteam: true,
		EnableCNSplit:    true,
		EnableSplitDNS:   true,
		IntranetDomains:  []string{"*.corp", "*.local", "*.lan", "*.internal", "*.home.arpa"},
	}
}

// BuildLazyPreset produces a validated RunSpec with curated SRS sets, app groups,
// split routing rules, and anti-leak split DNS.
func BuildLazyPreset(base spec.RunSpec, opts LazyPresetOptions) (spec.RunSpec, error) {
	next := base
	next.RuleSets = mergeRuleSets(base.RuleSets, presetRuleSets(opts))
	next.AppGroups = mergeAppGroups(base.AppGroups, presetAppGroups(opts))
	proxyTarget := presetProxyTarget(base)
	presetRules := buildPresetRules(opts, proxyTarget)
	next.Rules = assemblePresetRules(base.Rules, presetRules, opts.ReplaceExisting)
	if opts.EnableSplitDNS {
		next.DNS = buildPresetDNS(base.DNS, opts)
	}
	if err := next.Validate(); err != nil {
		return spec.RunSpec{}, err
	}
	return next, nil
}

func presetProxyTarget(s spec.RunSpec) string {
	if len(s.Groups) > 0 {
		return s.Groups[0].ID
	}
	return "proxy"
}

func presetRuleSets(opts LazyPresetOptions) []spec.RuleSet {
	byID := map[string]spec.RuleSet{}
	for _, rs := range CommunityRuleSets() {
		byID[rs.ID] = rs
	}
	var ids []string
	if opts.EnableAds {
		ids = append(ids, "ads")
	}
	if opts.EnableAI {
		ids = append(ids, "openai", "anthropic")
	}
	if opts.EnableAppleSteam {
		ids = append(ids, "apple-cn", "steam-cn")
	}
	result := make([]spec.RuleSet, 0, len(ids))
	for _, id := range ids {
		result = append(result, byID[id])
	}
	return result
}

func presetAppGroups(opts LazyPresetOptions) []spec.AppGroup {
	groups := []spec.AppGroup{
		{ID: "ag-cn-office", Name: "国内办公与通讯", Apps: []string{"WeChat.exe", "Weixin.exe", "QQ.exe", "DingTalk.exe", "Feishu.exe", "WXWork.exe", "wps.exe"}},
	}
	if opts.EnableAI {
		groups = append(groups, spec.AppGroup{
			ID: "ag-ai-dev", Name: "AI 与开发工具",
			Apps: []string{"Cursor.exe", "Code.exe", "Antigravity.exe", "git.exe", "gh.exe"},
		})
	}
	if opts.EnableAppleSteam {
		groups = append(groups, spec.AppGroup{
			ID: "ag-games", Name: "游戏平台",
			Apps: []string{"steam.exe", "steamwebhelper.exe", "EpicGamesLauncher.exe"},
		})
	}
	return groups
}

func buildPresetRules(opts LazyPresetOptions, proxyTarget string) []spec.Rule {
	var rules []spec.Rule
	if opts.EnableAds {
		rules = append(rules, spec.Rule{
			ID: "preset-ads-block", Name: "广告与跟踪拦截", Enabled: true,
			Apps: []string{}, Hosts: []string{"ruleset:ads"}, Ports: []string{},
			Network: "any", Action: "block", ResolveBefore: false,
		})
	}
	if domains := cleanDomains(opts.IntranetDomains); len(domains) > 0 {
		rules = append(rules, spec.Rule{
			ID: "preset-intranet-direct", Name: "公司内网 / 本地域名直连", Enabled: true,
			Apps: []string{}, Hosts: domains, Ports: []string{},
			Network: "any", Action: "direct", ResolveBefore: false,
		})
	}
	rules = append(rules, buildServicePresetRules(opts, proxyTarget)...)
	return rules
}

func buildServicePresetRules(opts LazyPresetOptions, proxyTarget string) []spec.Rule {
	var rules []spec.Rule
	if opts.EnableAI {
		rules = append(rules,
			spec.Rule{
				ID: "preset-ai-apps", Name: "AI 与开发工具代理", Enabled: true,
				Apps: []string{spec.AppGroupPrefix + "ag-ai-dev"}, Hosts: []string{}, Ports: []string{},
				Network: "any", Action: "proxy", TargetID: proxyTarget, ResolveBefore: false,
			},
			spec.Rule{
				ID: "preset-ai-domains", Name: "AI 服务分流 (OpenAI / Claude)", Enabled: true,
				Apps: []string{}, Hosts: []string{"ruleset:openai", "ruleset:anthropic"}, Ports: []string{},
				Network: "any", Action: "proxy", TargetID: proxyTarget, ResolveBefore: false,
			},
		)
	}
	rules = append(rules, spec.Rule{
		ID: "preset-cn-office", Name: "国内办公与通讯直连", Enabled: true,
		Apps: []string{spec.AppGroupPrefix + "ag-cn-office"}, Hosts: []string{}, Ports: []string{},
		Network: "any", Action: "direct", ResolveBefore: false,
	})
	if opts.EnableAppleSteam {
		rules = append(rules, spec.Rule{
			ID: "preset-apple-steam-cn", Name: "Apple / Steam 国内直连", Enabled: true,
			Apps: []string{}, Hosts: []string{"ruleset:apple-cn", "ruleset:steam-cn"}, Ports: []string{},
			Network: "any", Action: "direct", ResolveBefore: false,
		})
	}
	if opts.EnableCNSplit {
		rules = append(rules, buildCNSplitPresetRules(proxyTarget)...)
	}
	return rules
}

func buildCNSplitPresetRules(proxyTarget string) []spec.Rule {
	return []spec.Rule{
		{
			ID: "preset-cn-domains", Name: "中国大陆域名直连", Enabled: true,
			Apps: []string{}, Hosts: []string{"geosite:cn"}, Ports: []string{},
			Network: "any", Action: "direct", ResolveBefore: false,
		},
		{
			ID: "preset-non-cn-domains", Name: "海外域名代理", Enabled: true,
			Apps: []string{}, Hosts: []string{"geosite:geolocation-!cn"}, Ports: []string{},
			Network: "any", Action: "proxy", TargetID: proxyTarget, ResolveBefore: false,
		},
		{
			ID: "preset-cn-ip", Name: "中国大陆 IP 直连", Enabled: true,
			Apps: []string{}, Hosts: []string{"geoip:cn"}, Ports: []string{},
			Network: "any", Action: "direct", ResolveBefore: true,
		},
	}
}

func buildPresetDNS(base spec.DNS, opts LazyPresetOptions) spec.DNS {
	dns := base
	dns.Servers = slices.Clone(base.Servers)
	intranetTarget := "direct"
	if addr := strings.TrimSpace(opts.IntranetDNS); addr != "" {
		srv := spec.DNSServer{ID: "corp-intranet", Name: "公司内网 DNS", Address: addr, Detour: "direct"}
		dns.Servers = upsertDNSServer(dns.Servers, srv)
		intranetTarget = srv.ID
	}
	var rules []spec.DNSRule
	if domains := cleanDomains(opts.IntranetDomains); len(domains) > 0 {
		rules = append(rules, spec.DNSRule{
			ID: "dns-preset-intranet", Name: "内网域名定向解析", Enabled: true,
			Domains: domains, Server: intranetTarget,
		})
	}
	cnDomains := []string{}
	if opts.EnableCNSplit {
		cnDomains = append(cnDomains, "geosite:cn")
	}
	if opts.EnableAppleSteam {
		cnDomains = append(cnDomains, "ruleset:apple-cn", "ruleset:steam-cn")
	}
	if len(cnDomains) > 0 {
		rules = append(rules, spec.DNSRule{
			ID: "dns-preset-cn", Name: "国内域名走直连 DoH", Enabled: true,
			Domains: cnDomains, Server: "direct",
		})
	}
	dns.Rules = rules
	return dns
}

func assemblePresetRules(existing, preset []spec.Rule, replace bool) []spec.Rule {
	defaults := spec.DefaultRules()
	result := []spec.Rule{defaults[0]}
	if !replace {
		presetIDs := map[string]bool{}
		for _, r := range preset {
			presetIDs[r.ID] = true
		}
		for _, r := range existing {
			if !r.Pinned && r.ID != defaults[0].ID && r.ID != defaults[1].ID && !presetIDs[r.ID] {
				result = append(result, r)
			}
		}
	}
	result = append(result, preset...)
	return append(result, defaults[1])
}

func mergeRuleSets(existing, additions []spec.RuleSet) []spec.RuleSet {
	result := slices.Clone(existing)
	for _, add := range additions {
		idx := slices.IndexFunc(result, func(cur spec.RuleSet) bool { return cur.ID == add.ID })
		if idx >= 0 {
			result[idx] = add
		} else {
			result = append(result, add)
		}
	}
	return result
}

func mergeAppGroups(existing, additions []spec.AppGroup) []spec.AppGroup {
	result := slices.Clone(existing)
	for _, add := range additions {
		idx := slices.IndexFunc(result, func(cur spec.AppGroup) bool { return cur.ID == add.ID })
		if idx >= 0 {
			result[idx] = add
		} else {
			result = append(result, add)
		}
	}
	return result
}

func upsertDNSServer(existing []spec.DNSServer, srv spec.DNSServer) []spec.DNSServer {
	idx := slices.IndexFunc(existing, func(cur spec.DNSServer) bool { return cur.ID == srv.ID })
	if idx >= 0 {
		existing[idx] = srv
		return existing
	}
	return append(existing, srv)
}

func cleanDomains(items []string) []string {
	var result []string
	seen := map[string]bool{}
	for _, raw := range items {
		item := strings.TrimSpace(raw)
		if item == "" || seen[item] {
			continue
		}
		seen[item] = true
		result = append(result, item)
	}
	return result
}

var _ = errors.New
