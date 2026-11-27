package app

import (
	"context"
	"ferry/internal/convert"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestShareExportDoesNotIncludeSubscriptionURL(t *testing.T) {
	session, _ := emptySession()
	ctx := context.Background()
	sub, err := session.Subs().AddFromText(ctx, "test", "socks5://u:synthetic@127.0.0.1:1080")
	if err != nil {
		t.Fatal(err)
	}
	session.subs[0].url = "https://example.com/private-subscription-token"
	for _, format := range []string{"uri", "base64", "clash", "singbox"} {
		text, err := session.Subs().Export(ctx, sub.ID, format)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(text, "private-subscription-token") {
			t.Fatal("subscription access URL leaked into node export")
		}
		parsed, err := convert.ParseText(text)
		if err != nil || len(parsed.Nodes) != 1 {
			t.Fatal("export is not importable")
		}
	}
	path := filepath.Join(t.TempDir(), "links.txt")
	text, err := session.Subs().ExportNode(ctx, session.config.Nodes[0].ID, "uri")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
	other, _ := emptySession()
	if _, err := other.Subs().AddFromFile(ctx, path); err != nil {
		t.Fatal(err)
	}
}
