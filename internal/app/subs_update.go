package app

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"time"

	"ferry/internal/convert"
	"ferry/internal/ipc"
	"ferry/internal/spec"
)

func (s *Subs) Update(ctx context.Context, id string) (err error) {
	s.session.mu.Lock()
	state, revision := s.session.snapshot(), s.session.revision
	i := savedSubIndex(state, id)
	if i < 0 || state.Subs[i].URL == "" {
		s.session.mu.Unlock()
		return errors.New("订阅不存在或没有可更新的地址")
	}
	s.session.subs[i].view.Status = "updating"
	s.session.mu.Unlock()
	defer func() {
		if err != nil {
			s.updateError(id, err)
		}
	}()
	s.progress(id, "fetch", "")
	ua := effectiveSubUA(state.Subs[i].CustomUA, state.Settings.SubscriptionUA)
	body, headers, err := fetchSubscription(ctx, state.Subs[i].URL, ua)
	if err != nil {
		return err
	}
	s.progress(id, "parse", "")
	parsed, err := convert.ParseText(string(body))
	if err != nil {
		return err
	}
	if len(parsed.Skipped) > 0 {
		return errors.New("订阅包含不支持或无效的节点，已保留上次可用配置；请检查订阅内容")
	}
	s.progress(id, "apply", "")
	if err = s.applyUpdate(ctx, id, revision, parsed, headers); err != nil {
		return err
	}
	s.progress(id, "done", "")
	return nil
}

func (s *Subs) applyUpdate(ctx context.Context, id string, revision uint64, parsed convert.Result, headers http.Header) error {
	s.session.mu.Lock()
	defer s.session.mu.Unlock()
	if revision != s.session.revision {
		return errors.New("下载期间配置已变更，请重新更新订阅")
	}
	next := s.session.snapshot()
	i := savedSubIndex(next, id)
	if i < 0 {
		return errors.New("订阅已删除")
	}
	sub := next.Subs[i]
	other := slices.DeleteFunc(slices.Clone(next.Config.Nodes), func(node spec.Node) bool { return slices.Contains(sub.NodeIDs, node.ID) })
	fresh, skipped := withoutExisting(parsed, other)
	if len(fresh) == 0 {
		return noNodesError(skipped)
	}
	next.Config.Nodes = append(other, fresh...)
	next.Config.Groups = slices.DeleteFunc(slices.Clone(next.Config.Groups), func(group spec.Group) bool { return slices.Contains(sub.GroupIDs, group.ID) })
	groupIDs := mergeImportedGroups(&next.Config, parsed.Groups)
	if err := validateImportedConfig(next.Config); err != nil {
		return err
	}
	ua := effectiveSubUA(sub.CustomUA, next.Settings.SubscriptionUA)
	view := subscriptionMetadata(headers, sub.URL, ua, sub.View.AutoUpdate, sub.View.IntervalHours)
	view.ID, view.Name, view.NodeCount = id, sub.View.Name, len(fresh)
	view.GroupCount = len(groupIDs)
	view.LastUpdatedAt, view.Status = time.Now().UnixMilli(), "ok"
	next.Subs[i] = savedSubscription{View: view, NodeIDs: nodeIDs(fresh), GroupIDs: groupIDs, URL: sub.URL, CustomUA: sub.CustomUA}
	for _, removed := range sub.NodeIDs {
		if !slices.Contains(next.Subs[i].NodeIDs, removed) {
			delete(next.Favorites, removed)
		}
	}
	next.Selected = map[string]string{}
	return s.session.reconfigure(ctx, next)
}

func (s *Subs) updateError(id string, err error) {
	s.session.mu.Lock()
	for i := range s.session.subs {
		if s.session.subs[i].view.ID == id {
			s.session.subs[i].view.Status, s.session.subs[i].view.Error = "error", err.Error()
		}
	}
	s.session.mu.Unlock()
	s.progress(id, "error", err.Error())
}

func (s *Subs) UpdateAll(ctx context.Context) error {
	s.session.mu.Lock()
	var ids []string
	for _, sub := range s.session.subs {
		if sub.url != "" {
			ids = append(ids, sub.view.ID)
		}
	}
	s.session.mu.Unlock()
	var failures []error
	for _, id := range ids {
		if ctx.Err() != nil {
			return errors.New("订阅更新已取消")
		}
		if err := s.Update(ctx, id); err != nil {
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}

func (s *Session) RunBackground(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		now := time.Now()
		s.Subs().updateDue(ctx, now)
		s.syncRuleSetsIfDue(ctx, now)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (s *Session) syncRuleSetsIfDue(ctx context.Context, now time.Time) {
	dueIDs, prevBuiltin, prevCustom := s.collectDueRuleSets(now)
	if len(dueIDs) == 0 {
		return
	}
	if err := s.RuleSets().Update(ctx, dueIDs); err != nil {
		s.mu.Lock()
		s.lastRuleSetSync = prevBuiltin
		s.customSetSync = prevCustom
		s.mu.Unlock()
	}
}

func (s *Session) collectDueRuleSets(now time.Time) ([]string, int64, map[string]int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.remote.(ipc.RuleSetManager); !ok {
		return nil, 0, nil
	}
	prevBuiltin := s.lastRuleSetSync
	prevCustom := cloneSyncMap(s.customSetSync)
	globalHours := s.settings.RuleSetUpdateHours
	if globalHours <= 0 {
		globalHours = 24
	}
	var due []string
	if s.settings.RuleSetAutoUpdate && isDue(s.lastRuleSetSync, globalHours, now) {
		s.lastRuleSetSync = now.UnixMilli()
		due = append(due, spec.RuleSetCatalogIDs()...)
	}
	if s.customSetSync == nil {
		s.customSetSync = map[string]int64{}
	}
	for _, rs := range s.config.RuleSets {
		if !rs.AutoUpdate {
			continue
		}
		hours := rs.UpdateHours
		if hours <= 0 {
			hours = globalHours
		}
		if isDue(s.customSetSync[rs.ID], hours, now) {
			s.customSetSync[rs.ID] = now.UnixMilli()
			due = append(due, rs.ID)
		}
	}
	return due, prevBuiltin, prevCustom
}

func isDue(lastMs int64, hours int, now time.Time) bool {
	return lastMs == 0 || now.Sub(time.UnixMilli(lastMs)) >= time.Duration(hours)*time.Hour
}

func cloneSyncMap(src map[string]int64) map[string]int64 {
	if len(src) == 0 {
		return nil
	}
	out := make(map[string]int64, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out
}

func (s *Subs) updateDue(ctx context.Context, now time.Time) {
	s.session.mu.Lock()
	var ids []string
	for _, sub := range s.session.subs {
		if sub.url != "" && sub.view.AutoUpdate && sub.view.IntervalHours > 0 && now.Sub(time.UnixMilli(sub.view.LastUpdatedAt)) >= time.Duration(sub.view.IntervalHours)*time.Hour {
			ids = append(ids, sub.view.ID)
		}
	}
	s.session.mu.Unlock()
	for _, id := range ids {
		if ctx.Err() != nil {
			return
		}
		// Update emits a redacted error event and retains the last usable nodes.
		_ = s.Update(ctx, id)
	}
}
