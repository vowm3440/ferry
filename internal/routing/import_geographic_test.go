package routing

import (
	"ferry/internal/spec"
	"testing"
)

func TestClashGeographicRulesMapOnlyKnownCatalog(t *testing.T) {
	preview, err := Import("clash", "GEOSITE,cn,DIRECT\nGEOIP,cn,REJECT,no-resolve\nGEOIP,unknown,DIRECT", spec.Default())
	if err != nil || len(preview.Rules) != 2 || len(preview.Warnings) != 1 {
		t.Fatalf("catalog import: %+v %v", preview, err)
	}
	if preview.Rules[0].Hosts[0] != "geosite:cn" || preview.Rules[1].Hosts[0] != "geoip:cn" || preview.Rules[1].ResolveBefore {
		t.Fatal("geographic semantics changed")
	}
}
