package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"

	"ferry/internal/spec"
)

func (n *Nodes) SaveGroup(ctx context.Context, group Group) (Group, error) {
	specGrp, err := toSpecGroup(group)
	if err != nil {
		return Group{}, err
	}
	n.session.mu.Lock()
	defer n.session.mu.Unlock()
	next := n.session.snapshot()
	next.Config.Groups = upsertSpecGroup(next.Config.Groups, specGrp)
	if len(next.Config.Groups) > spec.MaxGroups {
		return Group{}, errors.New("分组数量超过限制")
	}
	if err := next.Config.Validate(); err != nil {
		return Group{}, err
	}
	if err := validateImportedConfig(next.Config); err != nil {
		return Group{}, err
	}
	if err := n.session.reconfigure(ctx, next); err != nil {
		return Group{}, err
	}
	interval, _ := time.ParseDuration(specGrp.Interval)
	return Group{
		ID:          specGrp.ID,
		Name:        specGrp.Name,
		Type:        specGrp.Type,
		Members:     slices.Clone(specGrp.Members),
		TestURL:     specGrp.TestURL,
		IntervalMin: int(interval.Minutes()),
	}, nil
}

func (n *Nodes) RemoveGroup(ctx context.Context, id string) error {
	id = strings.TrimSpace(id)
	n.session.mu.Lock()
	defer n.session.mu.Unlock()
	next := n.session.snapshot()
	idx := slices.IndexFunc(next.Config.Groups, func(g spec.Group) bool { return g.ID == id })
	if idx < 0 {
		if id == "proxy" {
			return errors.New("默认代理分组不可删除")
		}
		return errors.New("分组不存在")
	}
	next.Config.Groups = slices.Delete(slices.Clone(next.Config.Groups), idx, idx+1)
	for i := range next.Subs {
		next.Subs[i].GroupIDs = slices.DeleteFunc(slices.Clone(next.Subs[i].GroupIDs), func(gid string) bool { return gid == id })
		next.Subs[i].View.GroupCount = len(next.Subs[i].GroupIDs)
	}
	if err := next.Config.Validate(); err != nil {
		return errors.New("其它分组或规则仍引用此分组，请先移除对应引用")
	}
	delete(next.Selected, id)
	return n.session.reconfigure(ctx, next)
}

func toSpecGroup(group Group) (spec.Group, error) {
	name := strings.TrimSpace(group.Name)
	if name == "" || len(name) > 256 || strings.ContainsFunc(name, unicode.IsControl) {
		return spec.Group{}, errors.New("分组名称无效")
	}
	if group.Type != "select" && group.Type != "urltest" {
		return spec.Group{}, errors.New("分组类型仅支持手动选择或自动测速")
	}
	if len(group.Members) == 0 {
		return spec.Group{}, errors.New("请至少选择一个节点或子分组")
	}
	id := strings.TrimSpace(group.ID)
	if id == "" {
		var raw [8]byte
		if _, err := rand.Read(raw[:]); err != nil {
			return spec.Group{}, errors.New("无法生成分组标识")
		}
		id = "g-" + hex.EncodeToString(raw[:])
	}
	interval := ""
	testURL := ""
	if group.Type == "urltest" {
		mins := group.IntervalMin
		if mins <= 0 {
			mins = 5
		}
		if mins < 1 || mins > 1440 {
			return spec.Group{}, errors.New("测速间隔必须在 1 分钟到 24 小时之间")
		}
		interval = strconv.Itoa(mins) + "m"
		testURL = strings.TrimSpace(group.TestURL)
	}
	return spec.Group{
		ID:       id,
		Name:     name,
		Type:     group.Type,
		Members:  slices.Clone(group.Members),
		TestURL:  testURL,
		Interval: interval,
	}, nil
}

func upsertSpecGroup(groups []spec.Group, target spec.Group) []spec.Group {
	out := slices.Clone(groups)
	for i, existing := range out {
		if existing.ID == target.ID {
			out[i] = target
			return out
		}
	}
	return append(out, target)
}
