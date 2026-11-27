package app

import (
	"context"
	"ferry/internal/ipc"
)

type System struct{ session *Session }

func (s *Session) System() *System { return &System{session: s} }

// Capabilities advertises only implemented method groups so a newer UI can
// share a checkout with a backend that is still being upgraded.
func (s *System) Capabilities(context.Context) []string {
	s.session.mu.Lock()
	defer s.session.mu.Unlock()
	features := []string{
		"subs.text", "subs.url", "subs.ua", "nodes.groups", "share", "qr", "qr.screen", "deeplink",
		"rules", "rules.appgroups", "backup", "diagnostics", "share.lan",
		"settings.automation", "system.tools",
	}
	if s.session.deepLinkReg != nil {
		features = append(features, "deeplink.register")
	}
	if s.session.proxy != nil {
		features = append(features, "system.proxy")
	}
	if s.session.serviceCtrl != nil {
		features = append(features, "system.service", "system.tun")
	}
	if _, err := s.session.inspector(); err == nil {
		features = append(features, "core.latency", "connections", "logs")
	}
	if s.session.storage != nil {
		features = append(features, "settings.persist")
	}
	if _, ok := s.session.remote.(ipc.RuleSetManager); ok {
		features = append(features, "rulesets")
	}
	if _, err := s.session.resolveUpdateSource(); err == nil {
		features = append(features, "update.online")
	}
	return features
}
