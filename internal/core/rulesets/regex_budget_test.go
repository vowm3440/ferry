package rulesets

import (
	"context"
	"github.com/sagernet/sing-box/option"
	"net/http"
	"testing"
)

func TestRegexExpansionBudgetRejectsSmallButExpensiveSource(t *testing.T) {
	expressions := make([]string, 160)
	for i := range expressions {
		expressions[i] = "a{1000}"
	}
	data := writeSRS(t, option.PlainRuleSet{Rules: []option.HeadlessRule{{Type: "default", DefaultOptions: option.DefaultHeadlessRule{DomainRegex: expressions}}}})
	client := tlsClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.Write(data) }))
	if err := NewWithClient(t.TempDir(), client).Update(context.Background(), "geosite-cn"); err == nil {
		t.Fatal("regex expansion bypassed resource budget")
	}
}
