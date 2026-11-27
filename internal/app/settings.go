package app

import (
	"context"
	"errors"
	"slices"

	"ferry/internal/spec"
)

type Settings struct{ session *Session }

func defaultSettings() SettingsValue {
	v := SettingsValue{Language: "zh-CN", Theme: "system", SubscriptionUA: "clash.meta", SubscriptionUpdateHours: 24, UpdateChannel: "stable", ClashAPIMode: "random"}
	return fromSpec(v, spec.Default())
}

func fromSpec(v SettingsValue, s spec.RunSpec) SettingsValue {
	v.MixedPort = int(s.Inbound.MixedPort)
	v.AllowLAN = s.Inbound.AllowLAN
	v.Tun = TunSettings{Stack: s.Inbound.Tun.Stack, StrictRoute: s.Inbound.Tun.StrictRoute, Excludes: slices.Clone(s.Inbound.Tun.Excludes), MTU: s.Inbound.Tun.MTU, FakeIP: s.DNS.Mode == "fakeip"}
	if v.Tun.Excludes == nil {
		v.Tun.Excludes = []string{}
	}
	v.DNS = DNSSettings{Remote: s.DNS.Remote, Direct: s.DNS.Direct, Strategy: s.DNS.Strategy}
	v.LogLevel = s.Log.Level
	return v
}

func (s *Settings) Get(context.Context) (SettingsValue, error) {
	s.session.mu.Lock()
	defer s.session.mu.Unlock()
	v := s.session.settings
	v.Tun.Excludes = slices.Clone(v.Tun.Excludes)
	return v, nil
}

func (s *Settings) Set(ctx context.Context, v SettingsValue) error {
	if err := validateSettings(v); err != nil {
		return err
	}
	if v.Tun.Excludes == nil {
		v.Tun.Excludes = []string{}
	}
	s.session.mu.Lock()
	defer s.session.mu.Unlock()
	if err := s.session.requireStopped(ctx, "请先停止内核，再修改设置"); err != nil {
		return err
	}
	config := s.session.config
	config.Inbound.MixedPort = uint16(v.MixedPort)
	config.Inbound.AllowLAN = v.AllowLAN
	config.Inbound.Tun = spec.Tun{Stack: v.Tun.Stack, StrictRoute: v.Tun.StrictRoute, Excludes: slices.Clone(v.Tun.Excludes), MTU: v.Tun.MTU}
	config.DNS = spec.DNS{Mode: "redirhost", Remote: v.DNS.Remote, Direct: v.DNS.Direct, Strategy: v.DNS.Strategy}
	if v.Tun.FakeIP {
		config.DNS.Mode = "fakeip"
	}
	config.Log.Level = v.LogLevel
	if err := config.Validate(); err != nil {
		return errors.New("设置无效，请检查 DNS、端口和入站设置")
	}
	v.Tun.Excludes = slices.Clone(v.Tun.Excludes)
	next := s.session.snapshot()
	next.Config, next.Settings = config, v
	syncInheritedSubUA(&next)
	lightweightChanged, langChanged := v.Lightweight != s.session.settings.Lightweight, v.Language != s.session.settings.Language
	hook, langHook := s.session.lightweightHook, s.session.languageHook
	if err := s.commitSettingsWithAutoStart(next, v.AutoStart); err != nil {
		return err
	}
	if lightweightChanged && hook != nil {
		_ = hook(v.Lightweight)
	}
	if langChanged && langHook != nil {
		langHook(v.Language)
	}
	return nil
}

func (s *Settings) commitSettingsWithAutoStart(next savedSession, autoStart bool) error {
	if s.session.autoStart != nil && autoStart != s.session.settings.AutoStart {
		if err := s.session.autoStart.Set(autoStart); err != nil {
			return err
		}
		if err := s.session.commit(next); err != nil {
			_ = s.session.autoStart.Set(s.session.settings.AutoStart)
			return err
		}
		return nil
	}
	return s.session.commit(next)
}

func syncInheritedSubUA(state *savedSession) {
	for i := range state.Subs {
		if state.Subs[i].URL != "" && state.Subs[i].CustomUA == "" {
			state.Subs[i].View.UserAgent = state.Settings.SubscriptionUA
		}
	}
}

func validateSettings(v SettingsValue) error {
	if v.MixedPort < 1 || v.MixedPort > 65535 {
		return errors.New("代理端口必须在 1 到 65535 之间")
	}
	if v.Theme != "system" && v.Theme != "light" && v.Theme != "dark" {
		return errors.New("主题设置无效")
	}
	if v.Language != "zh-CN" && v.Language != "en-US" {
		return errors.New("语言设置无效")
	}
	if v.ClashAPIMode != "random" {
		return errors.New("内核控制接口必须使用随机端口")
	}
	if v.SubscriptionUpdateHours < 1 || v.SubscriptionUpdateHours > 720 || len(v.SubscriptionUA) > 256 {
		return errors.New("订阅设置无效")
	}
	if v.UpdateChannel != "stable" && v.UpdateChannel != "beta" {
		return errors.New("更新通道无效")
	}
	return nil
}
