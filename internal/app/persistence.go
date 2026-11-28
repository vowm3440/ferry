package app

import (
	"bytes"
	"encoding/json"
	"errors"
	"maps"
	"slices"
	"strings"

	"ferry/internal/spec"
	"ferry/internal/store"
)

type Storage interface {
	Load() ([]byte, error)
	Save([]byte) error
}

type savedSubscription struct {
	View     Subscription `json:"view"`
	NodeIDs  []string     `json:"nodeIds"`
	GroupIDs []string     `json:"groupIds,omitempty"`
	URL      string       `json:"url,omitempty"`
	CustomUA string       `json:"customUa,omitempty"`
}

type savedSession struct {
	Version   int                 `json:"version"`
	Config    spec.RunSpec        `json:"config"`
	Settings  SettingsValue       `json:"settings"`
	Favorites map[string]bool     `json:"favorites"`
	Subs      []savedSubscription `json:"subscriptions"`
	Selected  map[string]string   `json:"-"`
}

func NewPersistentSession(remote Remote, storage Storage) (*Session, error) {
	session := NewSession(remote)
	session.storage = storage
	data, err := storage.Load()
	if err != nil {
		return nil, errors.New("无法读取已保存配置；原文件未修改，请恢复备份后重试")
	}
	if len(data) == 0 {
		return session, nil
	}
	state, err := decodeSession(data)
	if err != nil {
		return nil, err
	}
	session.apply(state)
	return session, nil
}

// Caller holds session.mu and must replace shared slices before modifying
// their contents. Maps and subscription ownership lists are copied here.
func (s *Session) snapshot() savedSession {
	state := savedSession{Version: 1, Config: s.config, Settings: s.settings,
		Favorites: maps.Clone(s.favorites), Selected: maps.Clone(s.selected), Subs: []savedSubscription{}}
	state.Config.RuleSets = slices.Clone(s.config.RuleSets)
	state.Config.DNS.Servers = cloneDNSServers(s.config.DNS.Servers)
	state.Config.DNS.Rules = cloneDNSRules(s.config.DNS.Rules)
	state.Settings.Tun.Excludes = slices.Clone(s.settings.Tun.Excludes)
	state.Settings.DNS.Servers = cloneDNSServers(s.settings.DNS.Servers)
	state.Settings.DNS.Rules = cloneDNSRules(s.settings.DNS.Rules)
	for _, sub := range s.subs {
		state.Subs = append(state.Subs, savedSubscription{View: sub.view, NodeIDs: slices.Clone(sub.nodeIDs), GroupIDs: slices.Clone(sub.groupIDs), URL: sub.url, CustomUA: sub.customUA})
	}
	return state
}

func (s *Session) commit(state savedSession) error {
	if s.storage != nil {
		data, err := json.Marshal(state)
		if err != nil || len(data) > store.MaxBytes {
			return errors.New("配置无法编码或超过保存大小限制")
		}
		if err := s.storage.Save(data); err != nil {
			return errors.New("保存配置失败，未应用更改")
		}
	}
	s.apply(state)
	s.revision++
	return nil
}

func (s *Session) apply(state savedSession) {
	s.config, s.settings = state.Config, state.Settings
	s.favorites, s.selected = state.Favorites, state.Selected
	if s.favorites == nil {
		s.favorites = map[string]bool{}
	}
	if s.selected == nil {
		s.selected = map[string]string{}
	}
	s.subs = make([]subscription, 0, len(state.Subs))
	for _, sub := range state.Subs {
		s.subs = append(s.subs, subscription{view: sub.View, nodeIDs: sub.NodeIDs, groupIDs: sub.GroupIDs, url: sub.URL, customUA: sub.CustomUA})
	}
}

func decodeSession(data []byte) (savedSession, error) {
	var state savedSession
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if len(data) > store.MaxBytes || !json.Valid(data) || decoder.Decode(&state) != nil || state.Version != 1 {
		return state, errors.New("已保存配置的格式或版本无效")
	}
	configData, err := json.Marshal(state.Config)
	if err != nil {
		return state, errors.New("已保存配置无法读取")
	}
	if _, err := spec.Decode(configData); err != nil || validateSettings(state.Settings) != nil {
		return state, errors.New("已保存配置未通过校验")
	}
	if !validSavedSubscriptions(state) {
		return state, errors.New("已保存订阅数据不一致")
	}
	state.Settings = fromSpec(state.Settings, state.Config)
	return state, nil
}

func validSavedSubscriptions(state savedSession) bool {
	known, owned, ids := map[string]bool{}, map[string]bool{}, map[string]bool{}
	groups, ownedGroups := map[string]bool{}, map[string]bool{}
	for _, group := range state.Config.Groups {
		groups[group.ID] = true
	}
	for _, node := range state.Config.Nodes {
		known[node.ID] = true
	}
	for _, sub := range state.Subs {
		if sub.View.ID == "" || ids[sub.View.ID] || sub.View.NodeCount != len(sub.NodeIDs) || sub.View.GroupCount != len(sub.GroupIDs) {
			return false
		}
		if len(sub.CustomUA) > 256 || strings.ContainsAny(sub.CustomUA, "\r\n\x00") {
			return false
		}
		if sub.URL != "" {
			if _, err := subscriptionURL(sub.URL); err != nil {
				return false
			}
		}
		for _, id := range sub.GroupIDs {
			if !groups[id] || ownedGroups[id] {
				return false
			}
			ownedGroups[id] = true
		}
		ids[sub.View.ID] = true
		for _, id := range sub.NodeIDs {
			if !known[id] || owned[id] {
				return false
			}
			owned[id] = true
		}
	}
	return true
}
