package host

import (
	"context"
	"errors"

	"ferry/internal/core/rulesets"
	"ferry/internal/spec"
	"ferry/internal/telemetry"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common/json/badoption"
)

// NewWithRuleSets uses a trusted host-selected cache directory, never a path
// received through the GUI protocol. New() remains usable without a catalogue.
func NewWithRuleSets(root string) *Host { h := New(); h.ruleSets = rulesets.New(root); return h }

func (h *Host) RuleSetList(context.Context) ([]telemetry.RuleSetInfo, error) {
	if h.ruleSets == nil {
		return nil, errors.New("规则集缓存未初始化")
	}
	result := []telemetry.RuleSetInfo{}
	for _, v := range h.ruleSets.List() {
		result = append(result, telemetry.RuleSetInfo{ID: v.ID, Name: v.Name, URL: v.URL, UpdatedAt: v.UpdatedAt, SizeBytes: v.SizeBytes, Status: v.Status})
	}
	return result, nil
}

func (h *Host) RuleSetUpdate(ctx context.Context, id string) error {
	if h.ruleSets == nil {
		return errors.New("规则集缓存未初始化")
	}
	return h.ruleSets.Update(ctx, id)
}

func (h *Host) loadRuleSets(s spec.RunSpec, options *option.Options) error {
	for _, id := range s.ReferencedRuleSets() {
		if h.ruleSets == nil {
			return errors.New("规则引用的离线资源未安装，请先下载规则集")
		}
		plain, err := h.ruleSets.Load(id)
		if err != nil {
			return errors.New("离线规则集缺失或无效，请在规则集页面下载后重试")
		}
		options.Route.RuleSet = append(options.Route.RuleSet, option.RuleSet{Type: "inline", Tag: badoption.Listable[string]{id}, InlineOptions: plain})
	}
	return nil
}
