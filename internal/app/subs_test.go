package app

import (
	"context"
	"strings"
	"testing"

	"ferry/internal/ipc"
)

const subsUUID = "11111111-2222-4333-8444-555555555555"

func emptySession() (*Session, *remoteFixture) {
	remote := &remoteFixture{state: ipc.State{Status: "stopped", Mode: "rule"}}
	return NewSession(remote), remote
}

func TestSubsAddFromTextImportsNodesAndReportsSkips(t *testing.T) {
	session, remote := emptySession()
	subs, ctx := session.Subs(), context.Background()
	text := "vless://" + subsUUID + "@edge.example.com:443?security=tls&type=ws&path=%2Fws#US\n" +
		"vmess://eyJhZGQiOiJhIn0=\n" +
		"socks5://127.0.0.1:1080#local\n"
	sub, err := subs.AddFromText(ctx, "  我的节点 ", text)
	if err != nil {
		t.Fatal(err)
	}
	if sub.Name != "我的节点" || sub.NodeCount != 2 || sub.Status != "ok" || sub.URLMasked != "" || sub.LastUpdatedAt == 0 || !strings.HasPrefix(sub.ID, "s-") {
		t.Errorf("subscription = %+v", sub)
	}
	if len(sub.Skipped) != 1 || sub.Skipped[0].Line != 2 || !strings.Contains(sub.Skipped[0].Reason, "vmess") {
		t.Errorf("skipped = %+v", sub.Skipped)
	}
	list, err := (&Nodes{session}).List(ctx)
	if err != nil || len(list.Nodes) != 2 || len(list.Groups) != 1 || list.Nodes[0].Name != "US" || list.Nodes[0].Type != "vless" {
		t.Fatalf("node list = %+v err = %v", list, err)
	}
	if err := (&Core{session}).Start(ctx); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(remote.data), subsUUID) {
		t.Error("imported node missing from the RunSpec sent to the core")
	}
	listed, err := subs.List(ctx)
	if err != nil || len(listed) != 1 || listed[0].ID != sub.ID || len(listed[0].Skipped) != 0 {
		t.Errorf("list = %+v err = %v", listed, err)
	}
}

func TestSubsDefaultNameAndInvalidNames(t *testing.T) {
	session, _ := emptySession()
	subs, ctx := session.Subs(), context.Background()
	sub, err := subs.AddFromText(ctx, "   ", "socks5://127.0.0.1:1080")
	if err != nil || sub.Name != "手动导入" {
		t.Fatalf("default name: %+v %v", sub, err)
	}
	for _, name := range []string{strings.Repeat("名", 65), "a\nb"} {
		if _, err := subs.AddFromText(ctx, name, "socks5://127.0.0.1:1081"); err == nil {
			t.Errorf("name %.10q accepted", name)
		}
	}
}

func TestSubsRejectsWithoutChangingState(t *testing.T) {
	session, _ := emptySession()
	subs, ctx := session.Subs(), context.Background()
	secret := "vless://" + subsUUID + "@a.example.com:443?security=reality&pbk=KEYKEY"
	for _, text := range []string{"", "hello", secret} {
		_, err := subs.AddFromText(ctx, "x", text)
		if err == nil {
			t.Fatalf("%.12q: expected an error", text)
		}
		if strings.Contains(err.Error(), subsUUID) || strings.Contains(err.Error(), "KEYKEY") {
			t.Errorf("error leaks link content: %v", err)
		}
	}
	if _, err := subs.AddFromText(ctx, "x", secret); err == nil || !strings.Contains(err.Error(), "支持的范围") {
		t.Errorf("the only skip reason should be shown: %v", err)
	}
	if listed, _ := subs.List(ctx); len(listed) != 0 || len(session.config.Nodes) != 0 {
		t.Errorf("failed imports changed state: %+v", listed)
	}
}

func TestSubsDedupesAcrossImportsAndRemoves(t *testing.T) {
	session, _ := emptySession()
	subs, ctx := session.Subs(), context.Background()
	first, err := subs.AddFromText(ctx, "a", "socks5://127.0.0.1:1080#one\nsocks5://127.0.0.1:1081#two")
	if err != nil {
		t.Fatal(err)
	}
	second, err := subs.AddFromText(ctx, "b", "socks5://127.0.0.1:1080#again\nsocks5://127.0.0.1:1082#three")
	if err != nil {
		t.Fatal(err)
	}
	if second.NodeCount != 1 || len(second.Skipped) != 1 || !strings.Contains(second.Skipped[0].Reason, "已存在") {
		t.Errorf("second = %+v", second)
	}
	if _, err := subs.AddFromText(ctx, "c", "socks5://127.0.0.1:1082"); err == nil || !strings.Contains(err.Error(), "已存在") {
		t.Errorf("all-duplicate import: %v", err)
	}
	nodes := &Nodes{session}
	if err := nodes.SetFavorite(ctx, session.config.Nodes[0].ID, true); err != nil {
		t.Fatal(err)
	}
	if err := subs.Remove(ctx, first.ID); err != nil {
		t.Fatal(err)
	}
	list, _ := nodes.List(ctx)
	if len(list.Nodes) != 1 || list.Nodes[0].Name != "three" || len(session.favorites) != 0 {
		t.Errorf("after remove: %+v favorites=%v", list.Nodes, session.favorites)
	}
	if err := subs.Remove(ctx, first.ID); err == nil {
		t.Error("removing twice must fail")
	}
	if err := subs.Remove(ctx, second.ID); err != nil {
		t.Fatal(err)
	}
	if err := (&Core{session}).Start(ctx); err == nil {
		t.Error("core started with no nodes left")
	}
}

func TestSubsRefuseChangesWhileRunning(t *testing.T) {
	session, _ := emptySession()
	subs, ctx := session.Subs(), context.Background()
	sub, err := subs.AddFromText(ctx, "a", "socks5://127.0.0.1:1080")
	if err != nil {
		t.Fatal(err)
	}
	if err := (&Core{session}).Start(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := subs.AddFromText(ctx, "b", "socks5://127.0.0.1:1081"); err == nil || !strings.Contains(err.Error(), "停止") {
		t.Errorf("import while running: %v", err)
	}
	if err := subs.Remove(ctx, sub.ID); err == nil || !strings.Contains(err.Error(), "停止") {
		t.Errorf("remove while running: %v", err)
	}
}

func TestSubsAndDevConfigurationReplaceEachOther(t *testing.T) {
	session, _ := configured(t)
	subs, ctx := session.Subs(), context.Background()
	if _, err := subs.AddFromText(ctx, "a", "socks5://127.0.0.1:1081#new"); err != nil {
		t.Fatal(err)
	}
	list, _ := (&Nodes{session}).List(ctx)
	if len(list.Nodes) != 1 || list.Nodes[0].Name != "new" {
		t.Errorf("text import must replace a developer configuration: %+v", list.Nodes)
	}
	if session.config.Inbound.MixedPort != 7890 {
		t.Errorf("inbound settings must survive: %+v", session.config.Inbound)
	}
	other, _ := configured(t)
	if _, err := other.Subs().AddFromText(ctx, "a", "socks5://127.0.0.1:1081"); err != nil {
		t.Fatal(err)
	}
	spec := `{"nodes":[{"id":"test","tag":"t","outbound":{"type":"socks","server":"127.0.0.1","server_port":1080}}],"groups":[],"rules":[],"inbound":{"mixedPort":7890,"allowLan":false,"tun":{"enabled":false,"stack":"mixed","strictRoute":true,"excludes":[],"mtu":1500}},"dns":{"mode":"redirhost","remote":"https://1.1.1.1/dns-query","direct":"https://223.5.5.5/dns-query","strategy":"prefer_ipv4"},"mode":"rule","log":{"level":"info"}}`
	if err := (&Core{other}).Configure(ctx, spec); err != nil {
		t.Fatal(err)
	}
	if listed, _ := other.Subs().List(ctx); len(listed) != 0 {
		t.Errorf("developer configuration must clear subscriptions: %+v", listed)
	}
}
