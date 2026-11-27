package app

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"ferry/internal/ipc"
	"ferry/internal/store"
)

func TestSessionRestoresSubscriptionsSettingsAndFavorites(t *testing.T) {
	file := store.New(filepath.Join(t.TempDir(), "state.dat"))
	session, err := NewPersistentSession(&remoteFixture{}, file)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	sub, err := session.Subs().AddFromText(ctx, "persisted", "socks5://user:test-pass@127.0.0.1:1080#local")
	if err != nil {
		t.Fatal(err)
	}
	_, nodes, settings := session.Services()
	list, _ := nodes.List(ctx)
	if err := nodes.SetFavorite(ctx, list.Nodes[0].ID, true); err != nil {
		t.Fatal(err)
	}
	value, _ := settings.Get(ctx)
	value.Theme, value.MixedPort = "light", 7899
	if err := settings.Set(ctx, value); err != nil {
		t.Fatal(err)
	}
	restored, err := NewPersistentSession(&remoteFixture{state: ipc.State{Status: "stopped"}}, file)
	if err != nil {
		t.Fatal(err)
	}
	restoredSubs, _ := restored.Subs().List(ctx)
	core, restoredNodes, restoredSettings := restored.Services()
	restoredList, _ := restoredNodes.List(ctx)
	restoredValue, _ := restoredSettings.Get(ctx)
	if len(restoredSubs) != 1 || restoredSubs[0].ID != sub.ID || len(restoredList.Nodes) != 1 || !restoredList.Nodes[0].Favorite {
		t.Fatal("subscription or favorite did not survive a new session")
	}
	if restoredValue.Theme != "light" || restoredValue.MixedPort != 7899 {
		t.Fatal("settings did not survive a new session")
	}
	startAndStop(t, core)
	if err := restored.Subs().Remove(ctx, sub.ID); err != nil {
		t.Fatal(err)
	}
	again, err := NewPersistentSession(&remoteFixture{}, file)
	if err != nil || len(again.config.Nodes) != 0 || len(again.subs) != 0 {
		t.Fatal("removed subscription came back after reload")
	}
}

type failingStorage struct{}

func (failingStorage) Load() ([]byte, error) { return nil, nil }
func (failingStorage) Save([]byte) error     { return errors.New("synthetic disk failure") }

func TestFailedSaveDoesNotApplyMutation(t *testing.T) {
	session, err := NewPersistentSession(&remoteFixture{}, failingStorage{})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, err := session.Subs().AddFromText(ctx, "", "socks5://127.0.0.1:1080"); err == nil {
		t.Fatal("failed save reported successful import")
	}
	if len(session.config.Nodes) != 0 || len(session.subs) != 0 {
		t.Fatal("failed save changed live state")
	}
	_, _, settings := session.Services()
	value, _ := settings.Get(ctx)
	value.MixedPort = 7899
	if err := settings.Set(ctx, value); err == nil {
		t.Fatal("failed save reported successful settings update")
	}
	if session.config.Inbound.MixedPort != 7890 {
		t.Fatal("failed save changed the configured port")
	}
}
