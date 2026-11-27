package host

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/netip"
	"strconv"
	"strings"

	"ferry/internal/routing"
	"ferry/internal/spec"
	"ferry/internal/spec/policy"
	"ferry/internal/telemetry"
	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	sbrule "github.com/sagernet/sing-box/route/rule"
	corejson "github.com/sagernet/sing/common/json"
	M "github.com/sagernet/sing/common/metadata"
)

// TestRule is a simulation of rule mode. It does not dial destinations or make
// DNS requests; unresolved addresses are explicitly reported as indeterminate.
func (h *Host) TestRule(ctx context.Context, input telemetry.RuleTestRequest) (telemetry.RuleTestResult, error) {
	if err := input.Config.Validate(); err != nil {
		return telemetry.RuleTestResult{}, err
	}
	if !policy.Host(input.Host) || input.Port < 1 || input.Port > 65535 || len(input.App) > 32768 || strings.ContainsAny(input.App, "\x00\r\n") || (input.Network != "tcp" && input.Network != "udp") {
		return telemetry.RuleTestResult{}, errors.New("规则测试的应用、主机、端口或网络类型无效")
	}
	ctx, err := h.ruleTestContext(ctx, input.Config)
	if err != nil {
		return telemetry.RuleTestResult{}, err
	}
	defaults := spec.DefaultRules()
	rules := []spec.Rule{defaults[0]}
	for _, r := range input.Config.Rules {
		if !r.Pinned {
			rules = append(rules, r)
		}
	}
	rules = append(rules, defaults[1])
	for _, r := range rules {
		if ctx.Err() != nil {
			return telemetry.RuleTestResult{}, errCanceled
		}
		if !r.Enabled {
			continue
		}
		result, matched, err := evaluateRule(ctx, input, r)
		if err != nil {
			return telemetry.RuleTestResult{}, err
		}
		if matched {
			return result, nil
		}
	}
	return telemetry.RuleTestResult{}, errors.New("规则测试未找到默认规则")
}

func evaluateRule(ctx context.Context, input telemetry.RuleTestRequest, r spec.Rule) (telemetry.RuleTestResult, bool, error) {
	value, err := routing.Build(input.Config, r)
	if err != nil {
		return telemetry.RuleTestResult{}, false, err
	}
	matched := r.ID == "builtin-default"
	if !matched {
		matched, err = matchOptions(ctx, value, input)
	}
	if err != nil {
		return telemetry.RuleTestResult{}, false, err
	}
	if matched {
		return ruleResult(input.Config, r), true, nil
	}
	if _, err := netip.ParseAddr(input.Host); err != nil && routing.NeedsResolve(r) {
		resolve, err := routing.BuildResolve(input.Config, r)
		if err != nil {
			return telemetry.RuleTestResult{}, false, err
		}
		if resolve != nil {
			delete(resolve, "server")
			delete(resolve, "strategy")
			resolve["action"] = "route"
			resolve["outbound"] = "__direct"
			uncertain, err := matchOptions(ctx, resolve, input)
			if err != nil {
				return telemetry.RuleTestResult{}, false, err
			}
			if uncertain {
				return telemetry.RuleTestResult{MatchedRuleID: r.ID, MatchedRuleName: r.Name, Chain: []string{}, Stage: "dns-required"}, true, nil
			}
		}
	}
	return telemetry.RuleTestResult{}, false, nil
}

func matchOptions(ctx context.Context, value map[string]any, input telemetry.RuleTestRequest) (bool, error) {
	if err := resolveTestSets(ctx, value, input); err != nil {
		return false, err
	}
	// sing-box requires a condition even for an action-only match-all rule.
	if actionOnly(value) {
		value["network"] = []string{"tcp", "udp"}
	}
	data, err := json.Marshal(value)
	if err != nil {
		return false, errors.New("规则无法编码")
	}
	var options option.Rule
	if corejson.UnmarshalContext(ctx, data, &options) != nil {
		return false, errors.New("内核不支持此规则")
	}
	rule, err := sbrule.NewRule(ctx, log.NewNOPFactory().NewLogger("rule-test"), options, false)
	if err != nil {
		return false, errors.New("内核无法构造测试规则")
	}
	metadata := &adapter.InboundContext{Network: input.Network, Destination: M.ParseSocksaddr(net.JoinHostPort(input.Host, strconv.Itoa(input.Port))), ProcessInfo: &adapter.ConnectionOwner{ProcessPath: input.App}}
	if _, err := netip.ParseAddr(input.Host); err != nil {
		metadata.Domain = input.Host
	}
	matched := rule.Match(metadata)
	if err := rule.Close(); err != nil {
		return false, errors.New("规则测试清理失败")
	}
	return matched, nil
}

func actionOnly(value map[string]any) bool {
	for key := range value {
		if key != "action" && key != "outbound" {
			return false
		}
	}
	return true
}

func ruleResult(s spec.RunSpec, r spec.Rule) telemetry.RuleTestResult {
	tag := r.TargetID
	if r.Action == "direct" {
		tag = "__direct"
	}
	if r.Action == "block" {
		tag = "REJECT"
	}
	if r.Action == "proxy" {
		tag = routing.OutboundTag(s, r.TargetID)
	}
	if r.ID == spec.BuiltinDefaultRuleID && len(s.Groups) > 0 {
		tag = s.Groups[0].ID
	}
	return telemetry.RuleTestResult{MatchedRuleID: r.ID, MatchedRuleName: r.Name, Action: r.Action, OutboundTag: tag, Chain: []string{tag}, Stage: "matched"}
}
