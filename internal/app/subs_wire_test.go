package app

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

// A nil slice marshals to null, which the real core rejects. Cover each
// import transition through the decoder used by the receiving process.
func TestSubsConfigurationSurvivesCoreWireFormat(t *testing.T) {
	for _, initial := range []string{"empty", "developer"} {
		t.Run(initial, func(t *testing.T) {
			session, _ := emptySession()
			if initial == "developer" {
				session, _ = configured(t)
			}
			ctx := context.Background()
			core, _, _ := session.Services()
			subs := session.Subs()
			var ids []string
			for _, link := range []string{"socks5://127.0.0.1:1080#one", "socks5://127.0.0.1:1081#two"} {
				sub, err := subs.AddFromText(ctx, "", link)
				if err != nil {
					t.Fatal(err)
				}
				ids = append(ids, sub.ID)
				startAndStop(t, core)
			}
			if err := subs.Remove(ctx, ids[0]); err != nil {
				t.Fatal(err)
			}
			startAndStop(t, core)
			if err := subs.Remove(ctx, ids[1]); err != nil {
				t.Fatal(err)
			}
			if err := core.Start(ctx); err == nil {
				t.Fatal("core started after the last subscription was removed")
			}
			if _, err := subs.AddFromText(ctx, "", "socks5://127.0.0.1:1082"); err != nil {
				t.Fatal(err)
			}
			startAndStop(t, core)
		})
	}
}

func TestSettingsOmittedExcludesStillProducesRunnableConfiguration(t *testing.T) {
	session, _ := configured(t)
	core, _, settings := session.Services()
	value, err := settings.Get(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	value.Tun.Excludes = nil // omitted from a Wails call, equivalent to an empty list
	if err := settings.Set(context.Background(), value); err != nil {
		t.Fatal(err)
	}
	startAndStop(t, core)
	value, err = settings.Get(context.Background())
	if err != nil || value.Tun.Excludes == nil {
		t.Fatal("settings returned null instead of an empty excludes list")
	}
}

func TestSubsRejectsCombinedConfigurationTooLargeWithoutChangingState(t *testing.T) {
	session, _ := emptySession()
	ctx := context.Background()
	// Each batch is well below the input limit, but together their credentials
	// exceed the core's 1 MiB wire limit long before the node-count limit.
	batch := func(offset int) string {
		var links strings.Builder
		for i := 0; i < 80; i++ {
			fmt.Fprintf(&links, "trojan://%s@edge.example.com:%d\n", strings.Repeat("x", 7000), offset+i)
		}
		return links.String()
	}
	subs := session.Subs()
	first, err := subs.AddFromText(ctx, "first", batch(1000))
	if err != nil || first.NodeCount != 80 {
		t.Fatalf("first batch: %v", err)
	}
	if _, err := subs.AddFromText(ctx, "second", batch(2000)); err == nil {
		t.Fatal("accepted a combined configuration that cannot cross IPC")
	}
	listed, err := subs.List(ctx)
	if err != nil || len(listed) != 1 || listed[0].ID != first.ID {
		t.Fatal("failed import changed subscription state")
	}
	core, nodes, _ := session.Services()
	list, err := nodes.List(ctx)
	if err != nil || len(list.Nodes) != 80 {
		t.Fatal("failed import changed node state")
	}
	startAndStop(t, core)
}

func startAndStop(t *testing.T, core *Core) {
	t.Helper()
	if err := core.Start(context.Background()); err != nil {
		t.Fatalf("imported configuration cannot start the core: %v", err)
	}
	if err := core.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
}
