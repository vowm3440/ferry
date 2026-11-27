//go:build windows

package app

import (
	"context"
	"testing"
	"time"
)

func TestRulesApplyToRealCoreAndTesterWithoutExternalNetwork(t *testing.T) {
	session, process, port := importProcessSession(t)
	const target = "203.0.113.42:18080"
	link, hits := importSOCKSNode(t, target)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if _, err := session.Subs().AddFromText(ctx, "rules-test", link); err != nil {
		t.Fatal(err)
	}
	core, _, _ := session.Services()
	if err := core.Start(ctx); err != nil {
		t.Fatal(err)
	}
	client := importProxyClient(t, port)
	if body := importRequest(t, client, "http://"+target); body != "via-imported-node" {
		t.Fatal("default did not proxy")
	}
	rule := newRule("拒绝测试目标")
	rule.Hosts = []string{"203.0.113.42"}
	rule.Ports = []string{"18080"}
	rule.Action = "block"
	rule.ResolveBefore = false
	saved, err := session.Rules().Save(ctx, rule)
	if err != nil {
		t.Fatal(err)
	}
	result, err := session.Rules().Test(ctx, "test.exe", "203.0.113.42", 18080, "tcp")
	if err != nil || result.MatchedRuleID != saved.ID || result.Action != "block" {
		t.Fatalf("IPC rule tester: %+v %v", result, err)
	}
	response, err := client.Get("http://" + target)
	if err == nil {
		status := response.StatusCode
		response.Body.Close()
		if status < 400 {
			t.Fatalf("blocked request succeeded with HTTP %d", status)
		}
	}
	if hits.Load() != 1 {
		t.Fatal("blocked request reached proxy node")
	}
	if err := session.Rules().SetEnabled(ctx, saved.ID, false); err != nil {
		t.Fatal(err)
	}
	if body := importRequest(t, client, "http://"+target); body != "via-imported-node" || hits.Load() != 2 {
		t.Fatal("disabling rule did not restore default route")
	}
	list, _ := session.Rules().List(ctx)
	ids := []string{}
	for _, r := range list {
		ids = append(ids, r.ID)
	}
	if err := session.Rules().Apply(ctx, ids); err != nil {
		t.Fatal(err)
	}
	if err := core.Stop(ctx); err != nil {
		t.Fatal(err)
	}
	assertImportProcessClosed(t, process, port)
}
