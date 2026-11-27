package app

import (
	"context"
	"sync"

	"ferry/internal/convert"
	"ferry/internal/share"
)

type lanState struct {
	mu     sync.Mutex
	server *share.LAN
}
type LanShare struct {
	URL       string `json:"url"`
	ExpiresAt int64  `json:"expiresAt"`
	Active    bool   `json:"active"`
}

// Explicit user action only. This shares a snapshot of nodes/groups, never raw
// subscription access URLs or the mutable session store.
func (s *Subs) LanShareStart(ctx context.Context) (LanShare, error) {
	s.session.mu.Lock()
	payload, err := convert.Export(s.session.config.Nodes, s.session.config.Groups, "singbox")
	s.session.mu.Unlock()
	if err != nil {
		return LanShare{}, err
	}
	s.session.lan.mu.Lock()
	defer s.session.lan.mu.Unlock()
	if s.session.lan.server == nil {
		server, err := share.NewLAN(share.LANOptions{})
		if err != nil {
			return LanShare{}, err
		}
		s.session.lan.server = server
	}
	info, err := s.session.lan.server.Start(ctx, []byte(payload), "application/json")
	return lanInfo(info), err
}

func (s *Subs) LanShareStop(context.Context) error {
	s.session.lan.mu.Lock()
	defer s.session.lan.mu.Unlock()
	if s.session.lan.server == nil {
		return nil
	}
	return s.session.lan.server.Stop()
}

func (s *Subs) LanShareStatus(context.Context) LanShare {
	s.session.lan.mu.Lock()
	defer s.session.lan.mu.Unlock()
	if s.session.lan.server == nil {
		return LanShare{}
	}
	return lanInfo(s.session.lan.server.Status())
}

func lanInfo(info share.LANInfo) LanShare {
	result := LanShare{URL: info.URL, Active: info.Active}
	if !info.ExpiresAt.IsZero() {
		result.ExpiresAt = info.ExpiresAt.UnixMilli()
	}
	return result
}

func (s *Session) CloseShares() error {
	s.lan.mu.Lock()
	defer s.lan.mu.Unlock()
	if s.lan.server == nil {
		return nil
	}
	return s.lan.server.Close()
}
