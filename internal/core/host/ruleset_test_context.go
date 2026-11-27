package host

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"strconv"
	"strings"

	"ferry/internal/spec"
	"ferry/internal/telemetry"
	"github.com/sagernet/sing-box/adapter"
	sbrule "github.com/sagernet/sing-box/route/rule"
	M "github.com/sagernet/sing/common/metadata"
)

type ruleSetContextKey struct{}
type testRuleSets map[string][]adapter.HeadlessRule

func (h *Host) ruleTestContext(ctx context.Context, s spec.RunSpec) (context.Context, error) {
	sets := testRuleSets{}
	for _, id := range s.ReferencedRuleSets() {
		if h.ruleSets == nil {
			return nil, errors.New("规则集尚未安装")
		}
		plain, err := h.ruleSets.Load(id)
		if err != nil {
			return nil, errors.New("规则测试需要的离线资源缺失或无效，请先下载规则集")
		}
		for _, option := range plain.Rules {
			rule, err := sbrule.NewHeadlessRule(ctx, option)
			if err != nil {
				return nil, errors.New("规则集无法用于测试")
			}
			sets[id] = append(sets[id], rule)
		}
	}
	return context.WithValue(ctx, ruleSetContextKey{}, sets), nil
}

// Evaluate each set with the core's headless matcher, then replace just that
// condition by its truth value. This preserves outer OR/AND/invert semantics
// without requiring a live router or any network activity during simulation.
func resolveTestSets(ctx context.Context, value map[string]any, input telemetry.RuleTestRequest) error {
	if ids, ok := value["rule_set"].([]string); ok {
		matched, err := matchTestSets(ctx, ids, input)
		if err != nil {
			return err
		}
		inverted, _ := value["invert"].(bool)
		delete(value, "rule_set")
		value["port"] = []uint16{uint16(input.Port)}
		value["invert"] = inverted == matched
	}
	if children, ok := value["rules"].([]map[string]any); ok {
		for _, child := range children {
			if err := resolveTestSets(ctx, child, input); err != nil {
				return err
			}
		}
	}
	return nil
}

func matchTestSets(ctx context.Context, ids []string, input telemetry.RuleTestRequest) (bool, error) {
	sets, _ := ctx.Value(ruleSetContextKey{}).(testRuleSets)
	_, addressErr := netip.ParseAddr(input.Host)
	for _, id := range ids {
		rules, ok := sets[id]
		if !ok {
			return false, errors.New("规则测试缺少离线规则集")
		}
		if strings.HasPrefix(id, "geoip-") && addressErr != nil {
			continue
		}
		for _, rule := range rules {
			metadata := &adapter.InboundContext{Network: input.Network, Destination: M.ParseSocksaddr(net.JoinHostPort(input.Host, strconv.Itoa(input.Port)))}
			if addressErr != nil {
				metadata.Domain = input.Host
			}
			if rule.Match(metadata) && metadata.DeferredIPCIDRMatchGroups == 0 {
				return true, nil
			}
		}
	}
	return false, nil
}
