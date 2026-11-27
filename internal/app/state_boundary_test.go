package app

import (
	"context"
	"testing"
)

func TestMutationsRejectUnknownCoreState(t *testing.T) {
	for name, mutate := range map[string]func(*Session) error{
		"configure": func(s *Session) error { return (&Core{s}).Configure(context.Background(), `{}`) },
		"mode":      func(s *Session) error { return (&Core{s}).SetMode(context.Background(), "global") },
		"settings": func(s *Session) error {
			v := s.settings
			v.MixedPort = 7899
			return (&Settings{s}).Set(context.Background(), v)
		},
		"import": func(s *Session) error {
			_, err := s.Subs().AddFromText(context.Background(), "new", "socks5://127.0.0.1:9999")
			return err
		},
	} {
		t.Run(name, func(t *testing.T) {
			s, _ := configured(t)
			s.remote = &unavailableRemote{}
			if err := mutate(s); err == nil {
				t.Fatal("communication failure treated as stopped")
			}
			if s.config.Inbound.MixedPort != 7890 || s.config.Mode != "rule" || s.config.Nodes[0].ID != "test" {
				t.Fatal("failed state query changed configuration")
			}
		})
	}
}
