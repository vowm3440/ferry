package app

import (
	"encoding/json"
	"testing"
)

func TestSavedSubscriptionRejectsInvalidURLAndGroupOwnership(t *testing.T) {
	s, _ := configured(t)
	for name, mutate := range map[string]func(*savedSession){
		"bad URL": func(v *savedSession) {
			v.Subs = []savedSubscription{{View: Subscription{ID: "s", NodeCount: 1}, NodeIDs: []string{"test"}, URL: "file:///credentials"}}
		},
		"unknown group": func(v *savedSession) {
			v.Subs = []savedSubscription{{View: Subscription{ID: "s", NodeCount: 1, GroupCount: 1}, NodeIDs: []string{"test"}, GroupIDs: []string{"missing"}}}
		},
		"incorrect group count": func(v *savedSession) {
			v.Subs = []savedSubscription{{View: Subscription{ID: "s", NodeCount: 1, GroupCount: 2}, NodeIDs: []string{"test"}}}
		},
	} {
		t.Run(name, func(t *testing.T) {
			state := s.snapshot()
			mutate(&state)
			raw, err := json.Marshal(state)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := decodeSession(raw); err == nil {
				t.Fatal("invalid saved ownership accepted")
			}
		})
	}
}
