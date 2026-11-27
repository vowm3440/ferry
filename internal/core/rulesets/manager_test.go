package rulesets

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/sagernet/sing-box/common/srs"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
)

func TestCatalogReturnsIndependentFixedDefinitions(t *testing.T) {
	definitions := Catalog()
	if len(definitions) != 4 {
		t.Fatalf("catalog size = %d", len(definitions))
	}
	if definitions[1].ID != "geosite-non-cn" || definitions[1].URL != "https://raw.githubusercontent.com/SagerNet/sing-geosite/rule-set/geosite-geolocation-!cn.srs" {
		t.Fatalf("wrong non-cn mapping: %+v", definitions[1])
	}
	definitions[0].URL = "https://attacker.invalid/file"
	if Catalog()[0].URL != "https://raw.githubusercontent.com/SagerNet/sing-geosite/rule-set/geosite-cn.srs" {
		t.Fatal("caller mutated trusted catalog")
	}
}

func TestMissingCacheListsWithoutNetworkOrCreatingRoot(t *testing.T) {
	root := filepath.Join(t.TempDir(), "cache")
	m := NewWithClient(root, offlineClient(t))
	list := m.List()
	if len(list) != 4 || list[0].Status != "missing" || list[0].SizeBytes != 0 {
		t.Fatalf("unexpected missing metadata: %+v", list)
	}
	if _, err := m.Resolve("geosite-cn"); err == nil {
		t.Fatal("resolved a missing cache")
	}
	if _, err := os.Lstat(root); !os.IsNotExist(err) {
		t.Fatalf("read-only operation created cache: %v", err)
	}
}

func TestResolveUsesValidatedOfflineFile(t *testing.T) {
	root := t.TempDir()
	data := fixtureSRS(t, "example.cn")
	path := filepath.Join(root, "geosite-cn.srs")
	writeFixture(t, path, data)
	m := NewWithClient(root, offlineClient(t))
	resolved, err := m.Resolve("geosite-cn")
	if err != nil || resolved != path {
		t.Fatalf("Resolve = %q, %v", resolved, err)
	}
	info := m.List()[0]
	if info.Status != "ready" || info.SizeBytes != int64(len(data)) || info.UpdatedAt <= 0 {
		t.Fatalf("unexpected cached metadata: %+v", info)
	}
	raw, err := json.Marshal(info)
	if err != nil || !bytes.Contains(raw, []byte(`"updatedAt":`)) || bytes.Contains(raw, []byte(root)) {
		t.Fatalf("metadata must expose camelCase without local paths: %s, %v", raw, err)
	}
}

func TestLoadRetainsSnapshotAfterSuccessfulUpdate(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, filepath.Join(root, "geosite-cn.srs"), fixtureSRS(t, "old.example.cn"))
	data := fixtureSRS(t, "new.example.cn")
	client := tlsClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(data) }))
	m := NewWithClient(root, client)
	old, err := m.Load("geosite-cn")
	if err != nil {
		t.Fatal(err)
	}
	if err = m.Update(context.Background(), "geosite-cn"); err != nil {
		t.Fatal(err)
	}
	current, err := m.Load("geosite-cn")
	if err != nil {
		t.Fatal(err)
	}
	if !old.Rules[0].DefaultOptions.DomainMatcher.Match("old.example.cn") || old.Rules[0].DefaultOptions.DomainMatcher.Match("new.example.cn") {
		t.Fatal("update changed a running snapshot")
	}
	if !current.Rules[0].DefaultOptions.DomainMatcher.Match("new.example.cn") {
		t.Fatal("new snapshot did not load update")
	}
}

func TestPrivateIPCatalogUsesExistingFixedSource(t *testing.T) {
	if Catalog()[3].URL != "https://raw.githubusercontent.com/MetaCubeX/meta-rules-dat/sing/geo/geoip/private.srs" {
		t.Fatal("private IP catalog must use the verified existing resource")
	}
}

func TestUnknownIDsRejectBeforeNetwork(t *testing.T) {
	m := NewWithClient(t.TempDir(), offlineClient(t))
	for _, id := range []string{"", "../geosite-cn", `..\geoip-cn`, "geosite-cn.srs", "geosite-cn:ads", "GEOSITE-CN", "https://example.com/a.srs", `C:\a.srs`, "ruleset://geosite-cn"} {
		if _, err := m.Resolve(id); err == nil {
			t.Errorf("Resolve accepted %q", id)
		}
		if err := m.Update(context.Background(), id); err == nil {
			t.Errorf("Update accepted %q", id)
		}
	}
}

func fixtureSRS(t *testing.T, domain string) []byte {
	t.Helper()
	return writeSRS(t, option.PlainRuleSet{Rules: []option.HeadlessRule{{
		Type: C.RuleTypeDefault, DefaultOptions: option.DefaultHeadlessRule{Domain: []string{domain}},
	}}})
}

func writeSRS(t *testing.T, rules option.PlainRuleSet) []byte {
	t.Helper()
	var data bytes.Buffer
	if err := srs.Write(&data, rules, C.RuleSetVersionCurrent); err != nil {
		t.Fatal(err)
	}
	if _, err := srs.Read(bytes.NewReader(data.Bytes()), false); err != nil {
		t.Fatalf("invalid test fixture: %v", err)
	}
	return data.Bytes()
}

func writeFixture(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func offlineClient(t *testing.T) *http.Client {
	t.Helper()
	return &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		t.Error("offline operation attempted network")
		return nil, context.Canceled
	})}
}
