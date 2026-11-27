package rulesets

import (
	"bytes"
	"compress/zlib"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"testing"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
)

func TestRejectsInvalidSRSWithoutReplacingCache(t *testing.T) {
	old := fixtureSRS(t, "old.example.cn")
	cases := map[string][]byte{
		"empty":               {},
		"bad-magic":           []byte("JSON"),
		"future-version":      append([]byte{'S', 'R', 'S', 255}, old[4:]...),
		"zero-version":        append([]byte{'S', 'R', 'S', 0}, old[4:]...),
		"checksum":            append(bytes.Clone(old[:len(old)-1]), old[len(old)-1]^255),
		"compressed-trailing": append(bytes.Clone(old), 0),
		"payload-trailing":    compressedSRS(t, []byte{0, 0}),
		"huge-rules":          compressedSRS(t, binary.AppendUvarint(nil, 1<<40)),
		"huge-string":         compressedSRS(t, append([]byte{1, 0, 3, 1}, binary.AppendUvarint(nil, 1<<40)...)),
		"unknown-item":        compressedSRS(t, []byte{1, 0, 254}),
		"non-geographic":      writeSRS(t, option.PlainRuleSet{Rules: []option.HeadlessRule{{Type: C.RuleTypeDefault, DefaultOptions: option.DefaultHeadlessRule{ProcessPath: []string{`C:\secret`}}}}}),
	}
	for name, bad := range cases {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "geosite-cn.srs")
			writeFixture(t, path, old)
			client := tlsClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(bad) }))
			m := NewWithClient(root, client)
			if err := m.Update(context.Background(), "geosite-cn"); err == nil {
				t.Fatal("invalid SRS accepted")
			}
			assertFileBytes(t, path, old)
			writeFixture(t, path, bad)
			if _, err := m.Resolve("geosite-cn"); err == nil {
				t.Fatal("invalid local SRS resolved")
			}
		})
	}
}

func TestDecompressionBombIsRejected(t *testing.T) {
	var data bytes.Buffer
	data.Write([]byte{'S', 'R', 'S', C.RuleSetVersionCurrent})
	writer := zlib.NewWriter(&data)
	if _, err := io.CopyN(writer, zeroReader{}, 64<<20+1); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	old := fixtureSRS(t, "old.example.cn")
	path := filepath.Join(root, "geosite-cn.srs")
	writeFixture(t, path, old)
	client := tlsClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(data.Bytes()) }))
	if err := NewWithClient(root, client).Update(context.Background(), "geosite-cn"); err == nil {
		t.Fatal("decompression bomb accepted")
	}
	assertFileBytes(t, path, old)
}

func TestGeographicLogicalRulesAreAccepted(t *testing.T) {
	rules := option.PlainRuleSet{Rules: []option.HeadlessRule{{Type: C.RuleTypeLogical, LogicalOptions: option.LogicalHeadlessRule{
		Mode: C.LogicalTypeOr, Rules: []option.HeadlessRule{
			{Type: C.RuleTypeDefault, DefaultOptions: option.DefaultHeadlessRule{DomainSuffix: []string{"example.cn"}, DomainKeyword: []string{"example"}, DomainRegex: []string{`.*\.cn$`}}},
			{Type: C.RuleTypeDefault, DefaultOptions: option.DefaultHeadlessRule{IPCIDR: []string{"10.0.0.0/8", "fd00::/8"}}},
		},
	}}}}
	root := t.TempDir()
	writeFixture(t, filepath.Join(root, "geoip-cn.srs"), writeSRS(t, rules))
	if _, err := New(root).Resolve("geoip-cn"); err != nil {
		t.Fatal(err)
	}
}

func TestCancelledValidationStopsBeforeDecoding(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := validateSRSContext(ctx, fixtureSRS(t, "example.cn")); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled validation = %v", err)
	}
}

func compressedSRS(t *testing.T, payload []byte) []byte {
	t.Helper()
	var data bytes.Buffer
	data.Write([]byte{'S', 'R', 'S', C.RuleSetVersionCurrent})
	writer := zlib.NewWriter(&data)
	if _, err := writer.Write(payload); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return data.Bytes()
}
