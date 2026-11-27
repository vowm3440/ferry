package app

import (
	"context"
	"errors"
	"slices"
	"strings"

	"ferry/internal/convert"
)

func (s *Subs) AddFromURL(ctx context.Context, name, raw string, autoUpdate bool) (Subscription, error) {
	if _, err := subscriptionURL(raw); err != nil {
		return Subscription{}, err
	}
	s.session.mu.Lock()
	settings := s.session.settings
	s.session.mu.Unlock()
	body, headers, err := fetchSubscription(ctx, raw, settings.SubscriptionUA)
	if err != nil {
		return Subscription{}, err
	}
	parsed, err := convert.ParseText(string(body))
	if err != nil {
		return Subscription{}, err
	}
	if strings.TrimSpace(name) == "" {
		name = profileTitle(headers)
	}
	name, err = subName(name)
	if err != nil {
		return Subscription{}, err
	}
	view := subscriptionMetadata(headers, raw, settings.SubscriptionUA, autoUpdate, settings.SubscriptionUpdateHours)
	return s.add(ctx, name, parsed, strings.TrimSpace(raw), view)
}

func (s *Subs) Rename(_ context.Context, id, name string) error {
	name, err := subName(name)
	if err != nil {
		return err
	}
	s.session.mu.Lock()
	defer s.session.mu.Unlock()
	next := s.session.snapshot()
	i := savedSubIndex(next, id)
	if i < 0 {
		return errors.New("订阅不存在")
	}
	next.Subs[i].View.Name = name
	return s.session.commit(next)
}

func (s *Subs) SetAutoUpdate(_ context.Context, id string, on bool, hours int) error {
	if hours < 1 || hours > 720 {
		return errors.New("订阅更新间隔须在 1 到 720 小时之间")
	}
	s.session.mu.Lock()
	defer s.session.mu.Unlock()
	next := s.session.snapshot()
	i := savedSubIndex(next, id)
	if i < 0 {
		return errors.New("订阅不存在")
	}
	if on && next.Subs[i].URL == "" {
		return errors.New("手动导入的节点没有订阅地址，无法自动更新")
	}
	next.Subs[i].View.AutoUpdate, next.Subs[i].View.IntervalHours = on, hours
	return s.session.commit(next)
}

func (s *Subs) SetUserAgent(_ context.Context, id, ua string) error {
	ua = strings.TrimSpace(ua)
	if len(ua) > 256 || strings.ContainsAny(ua, "\r\n\x00") {
		return errors.New("订阅 User-Agent 无效")
	}
	s.session.mu.Lock()
	defer s.session.mu.Unlock()
	next := s.session.snapshot()
	i := savedSubIndex(next, id)
	if i < 0 {
		return errors.New("订阅不存在")
	}
	if next.Subs[i].URL == "" {
		return errors.New("手动导入的节点没有订阅地址，无法设置 User-Agent")
	}
	next.Subs[i].CustomUA = ua
	next.Subs[i].View.UserAgent = effectiveSubUA(ua, next.Settings.SubscriptionUA)
	return s.session.commit(next)
}

func effectiveSubUA(custom, global string) string {
	if custom != "" {
		return custom
	}
	return global
}

func savedSubIndex(state savedSession, id string) int {
	return slices.IndexFunc(state.Subs, func(sub savedSubscription) bool { return sub.View.ID == id })
}
