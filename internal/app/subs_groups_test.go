package app

import (
	"context"
	"testing"
)

func TestDocumentImportRetainsAndRemovesOwnedGroups(t *testing.T) {
	session, _ := emptySession()
	ctx := context.Background()
	sub, err := session.Subs().AddFromText(ctx, "document", `{"outbounds":[{"tag":"one","type":"socks","server":"127.0.0.1","server_port":1080},{"tag":"Configured","type":"selector","outbounds":["one","DIRECT"]}]}`)
	if err != nil {
		t.Fatal(err)
	}
	core, nodes, _ := session.Services()
	list, _ := nodes.List(ctx)
	if sub.GroupCount != 1 || len(list.Groups) != 1 || list.Groups[0].Name != "Configured" {
		t.Fatal("document groups replaced by a synthetic group")
	}
	startAndStop(t, core)
	if err := session.Subs().Remove(ctx, sub.ID); err != nil {
		t.Fatal(err)
	}
	list, _ = nodes.List(ctx)
	if len(list.Nodes) != 0 || len(list.Groups) != 0 {
		t.Fatal("owned groups survived removal")
	}
}
