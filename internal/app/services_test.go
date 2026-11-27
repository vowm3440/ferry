package app

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"ferry/internal/ipc"
	"ferry/internal/spec"
)

type remoteFixture struct {
	state  ipc.State
	starts int
	data   json.RawMessage
}

func (r *remoteFixture) Status(context.Context) (ipc.State, error) {
	state := r.state
	if state.Status == "" {
		state.Status = "stopped"
	}
	return state, nil
}
func (r *remoteFixture) Start(_ context.Context, data json.RawMessage) error {
	if _, err := spec.Decode(data); err != nil {
		return err
	}
	r.starts++
	r.data = data
	r.state.Status = "running"
	return nil
}
func (r *remoteFixture) Stop(context.Context) error { r.state.Status = "stopped"; return nil }
func (r *remoteFixture) SetMode(_ context.Context, mode string) error {
	r.state.Mode = mode
	return nil
}
func (r *remoteFixture) SelectNode(context.Context, string, string) error { return nil }
func (r *remoteFixture) Snapshot(context.Context) (ipc.Traffic, error) {
	return ipc.Traffic{UpHistory: make([]int64, 60), DownHistory: make([]int64, 60)}, nil
}

func configured(t *testing.T) (*Session, *remoteFixture) {
	t.Helper()
	remote := &remoteFixture{state: ipc.State{Status: "stopped", Mode: "rule"}}
	session := NewSession(remote)
	err := (&Core{session}).Configure(context.Background(), `{"nodes":[{"id":"test","tag":"本机测试","outbound":{"type":"socks","server":"127.0.0.1","server_port":1080}}],"groups":[],"rules":[],"inbound":{"mixedPort":7890,"allowLan":false,"tun":{"enabled":false,"stack":"mixed","strictRoute":true,"excludes":[],"mtu":1500}},"dns":{"mode":"redirhost","remote":"https://1.1.1.1/dns-query","direct":"https://223.5.5.5/dns-query","strategy":"prefer_ipv4"},"mode":"rule","log":{"level":"info"}}`)
	if err != nil {
		t.Fatal(err)
	}
	return session, remote
}

func TestCoreRefusesEmptyAndInvalidConfigurations(t *testing.T) {
	r := &remoteFixture{state: ipc.State{Status: "stopped"}}
	c := &Core{NewSession(r)}
	if err := c.Start(context.Background()); err == nil || r.starts != 0 {
		t.Fatal("empty configuration started core")
	}
	for _, input := range []string{`{"unknown":"secret"}`, `{`, strings.Repeat("x", ipc.MaxBodySize+1)} {
		err := c.Configure(context.Background(), input)
		if err == nil || strings.Contains(err.Error(), "secret") {
			t.Fatalf("invalid configuration error: %v", err)
		}
	}
}

func TestCoreLifecycleUsesConfiguredNodeAndRejectsLiveEdits(t *testing.T) {
	s, r := configured(t)
	c := &Core{s}
	ctx := context.Background()
	if err := c.Start(ctx); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(r.data), "本机测试") {
		t.Fatal("configured node missing from start")
	}
	if err := c.Configure(ctx, string(r.data)); err == nil {
		t.Fatal("silently replaced live configuration")
	}
	if err := c.SetMode(ctx, "global"); err != nil {
		t.Fatal(err)
	}
	if c.Status(ctx).Mode != "global" {
		t.Fatal("mode not visible")
	}
	if err := c.Stop(ctx); err != nil || c.Status(ctx).Status != "stopped" {
		t.Fatal("failed to stop")
	}
}

func TestSettingsRejectInvalidAndDoNotAliasInternalState(t *testing.T) {
	s, _ := configured(t)
	settings := &Settings{s}
	ctx := context.Background()
	v, err := settings.Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	v.MixedPort = 0
	if settings.Set(ctx, v) == nil {
		t.Fatal("invalid port accepted")
	}
	v, _ = settings.Get(ctx)
	v.MixedPort = 7899
	v.Theme = "light"
	if err = settings.Set(ctx, v); err != nil {
		t.Fatal(err)
	}
	v, _ = settings.Get(ctx)
	if v.MixedPort != 7899 || v.Theme != "light" {
		t.Fatal("settings did not apply")
	}
	if len(v.Tun.Excludes) > 0 {
		v.Tun.Excludes[0] = "tampered"
	}
	next, _ := settings.Get(ctx)
	if len(next.Tun.Excludes) > 0 && next.Tun.Excludes[0] == "tampered" {
		t.Fatal("settings slices alias mutable state")
	}
}

func TestNodeListDoesNotExposeCredentials(t *testing.T) {
	s, _ := configured(t)
	list, err := (&Nodes{s}).List(context.Background())
	if err != nil || len(list.Nodes) != 1 || list.Nodes[0].Server != "127.0.0.1" {
		t.Fatalf("node list: %+v %v", list, err)
	}
	if list.Nodes[0].LatencyMs != -1 || list.Nodes[0].LatencyStatus != "idle" {
		t.Fatal("invented latency")
	}
	if len(list.Groups) != 1 || list.Groups[0].ID != "proxy" || list.Nodes[0].GroupID != "proxy" || !list.Nodes[0].Selected {
		t.Fatal("ungrouped nodes must use the core's default selector")
	}
	if err := (&Nodes{s}).SetFavorite(context.Background(), "missing", true); err == nil {
		t.Fatal("unknown node favorite accepted")
	}
}

func TestCoreStartIsIdempotentAndRestartClearsStaleSelections(t *testing.T) {
	s, r := configured(t)
	c := &Core{s}
	ctx := context.Background()
	if err := c.Start(ctx); err != nil {
		t.Fatal(err)
	}
	if err := c.Start(ctx); err != nil || r.starts != 1 {
		t.Fatal("duplicate Start invoked the backend")
	}
	s.selected["proxy"] = "stale-node"
	if err := c.Restart(ctx); err != nil || r.starts != 2 {
		t.Fatal("restart did not create a new instance")
	}
	if len(s.selected) != 0 {
		t.Fatal("restarted core still displays obsolete selections")
	}
}

func TestModeBeforeStartAndFavoriteUpdate(t *testing.T) {
	s, r := configured(t)
	core, nodes, settings := s.Services()
	ctx := context.Background()
	if err := core.SetMode(ctx, "global"); err != nil || r.state.Mode != "rule" || core.Status(ctx).Mode != "global" {
		t.Fatal("stopped-mode change should prepare the next start")
	}
	if err := core.SetMode(ctx, "invalid"); err == nil {
		t.Fatal("invalid mode accepted")
	}
	if err := core.SelectNode(ctx, "proxy", "test"); err == nil {
		t.Fatal("selection before start accepted")
	}
	if err := nodes.SetFavorite(ctx, "test", true); err != nil {
		t.Fatal(err)
	}
	list, err := nodes.List(ctx)
	if err != nil || !list.Nodes[0].Favorite {
		t.Fatal("favorite update missing")
	}
	if _, err := core.Snapshot(ctx); err != nil {
		t.Fatal(err)
	}
	if err := core.Start(ctx); err != nil {
		t.Fatal(err)
	}
	v, _ := settings.Get(ctx)
	if err := settings.Set(ctx, v); err == nil {
		t.Fatal("settings silently changed while running")
	}
}

func TestSettingsValidationPreservesPreviousConfiguration(t *testing.T) {
	s, _ := configured(t)
	settings := &Settings{s}
	ctx := context.Background()
	for _, change := range []func(*SettingsValue){
		func(v *SettingsValue) { v.Theme = "invalid" }, func(v *SettingsValue) { v.Language = "invalid" },
		func(v *SettingsValue) { v.ClashAPIMode = "fixed" }, func(v *SettingsValue) { v.MixedPort = 0 },
		func(v *SettingsValue) { v.SubscriptionUpdateHours = 0 }, func(v *SettingsValue) { v.UpdateChannel = "invalid" },
		func(v *SettingsValue) { v.DNS.Remote = "file:///secret" },
	} {
		value, _ := settings.Get(ctx)
		change(&value)
		if err := settings.Set(ctx, value); err == nil {
			t.Fatal("invalid settings accepted")
		}
		current, _ := settings.Get(ctx)
		if current.MixedPort != 7890 || current.Theme != "system" || current.AutoStart {
			t.Fatal("failed settings update changed state")
		}
	}
}
