package rulesets

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"ferry/internal/spec"
)

func TestCustomRuleSetSyncDownloadLoadAndDelete(t *testing.T) {
	data := fixtureSRS(t, "api.openai.com")
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rules/openai.srs" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write(data)
	}))
	defer server.Close()
	target, _ := url.Parse(server.URL)
	transport := server.Client().Transport
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		clone := r.Clone(r.Context())
		clone.URL.Host = target.Host
		clone.Host = target.Host
		return transport.RoundTrip(clone)
	})}

	root := filepath.Join(t.TempDir(), "rulesets")
	m := NewWithClient(root, client)
	custom := []spec.RuleSet{{ID: "openai", Name: "OpenAI", URL: "https://cdn.example.invalid/rules/openai.srs", AutoUpdate: true, UpdateHours: 12}}
	if err := m.SyncCustom(custom); err != nil {
		t.Fatal(err)
	}
	if err := m.Update(context.Background(), "openai"); err != nil {
		t.Fatalf("custom update failed: %v", err)
	}
	plain, err := m.Load("openai")
	if err != nil || !plain.Rules[0].DefaultOptions.DomainMatcher.Match("api.openai.com") {
		t.Fatalf("custom Load failed: %v", err)
	}
	list := m.List()
	if len(list) != 5 || !list[4].Custom || list[4].Status != "ready" || list[4].UpdateHours != 12 {
		t.Fatalf("unexpected custom list entry: %+v", list)
	}
	if err := m.Delete("geosite-cn"); err == nil {
		t.Fatal("allowed deleting built-in rule-set")
	}
	if err := m.Delete("openai"); err != nil {
		t.Fatalf("Delete custom failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "openai.srs")); !os.IsNotExist(err) {
		t.Fatal("cached custom srs file still exists after Delete")
	}
}
