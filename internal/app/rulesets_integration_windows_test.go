//go:build windows

package app

import (
	"bytes"
	"context"
	"ferry/internal/spec"
	"github.com/sagernet/sing-box/common/srs"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRuleSetCacheThroughRealProcess(t *testing.T) {
	root := t.TempDir()
	t.Setenv("FERRY_CONFIG_DIR", root)
	if err := os.Mkdir(filepath.Join(root, "rulesets"), 0700); err != nil {
		t.Fatal(err)
	}
	var data bytes.Buffer
	if err := srs.Write(&data, option.PlainRuleSet{Rules: []option.HeadlessRule{{Type: "default", DefaultOptions: option.DefaultHeadlessRule{Domain: []string{"example.cn"}}}}}, C.RuleSetVersionCurrent); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "rulesets", "geosite-cn.srs"), data.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	s, process, port := importProcessSession(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	list, err := s.RuleSets().List(ctx, true)
	if err != nil || len(list) != 4 || list[0].Status != "ready" {
		t.Fatalf("catalogue: %+v %v", list, err)
	}
	if _, err := s.Subs().AddFromText(ctx, "test", "socks5://127.0.0.1:1080"); err != nil {
		t.Fatal(err)
	}
	rule, err := s.Rules().Save(ctx, spec.Rule{Name: "Geo test", Enabled: true, Hosts: []string{"geosite:cn"}, Action: "block", Network: "any"})
	if err != nil {
		t.Fatal(err)
	}
	if err := (&Core{s}).Start(ctx); err != nil {
		t.Fatal(err)
	}
	result, err := s.Rules().Test(ctx, "app.exe", "example.cn", 443, "tcp")
	if err != nil || result.MatchedRuleID != rule.ID {
		t.Fatalf("cached set match: %+v %v", result, err)
	}
	if err := s.RuleSets().Update(ctx, []string{"../escape"}); err == nil {
		t.Fatal("unsafe catalogue ID accepted")
	}
	if err := (&Core{s}).Stop(ctx); err != nil {
		t.Fatal(err)
	}
	assertImportProcessClosed(t, process, port)
}
