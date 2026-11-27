package app

import (
	"context"
	"strings"
	"testing"
)

func TestQRCodeImportAndRulePreviewAreUsableThroughServices(t *testing.T) {
	s, _ := emptySession()
	ctx := context.Background()
	const link = "socks5://test-user:test-password@127.0.0.1:1080#local"
	encoded, err := s.Subs().QRPNG(ctx, link)
	if err != nil {
		t.Fatal(err)
	}
	imports, err := s.Subs().ImportFromQR(ctx, "data:image/png;base64,"+encoded)
	if err != nil || len(imports) != 1 || len(s.config.Nodes) != 1 {
		t.Fatalf("QR service import: %v", err)
	}
	svg, err := s.Subs().QRSvg(ctx, link, 256)
	if err != nil || !strings.Contains(svg, "<svg") {
		t.Fatal("QR SVG not created")
	}
	preview, err := s.Rules().Import(ctx, "clash", "DOMAIN,example.com,REJECT\nRULE-SET,unsupported,DIRECT")
	if err != nil || len(preview.Rules) != 1 || len(preview.Warnings) != 1 {
		t.Fatalf("import preview: %+v %v", preview, err)
	}
	before, _ := s.Rules().List(ctx)
	if len(before) != 2 {
		t.Fatal("preview applied rules before confirmation")
	}
	if _, err := s.Rules().Save(ctx, preview.Rules[0]); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Subs().ImportFromQR(ctx, "bad-base64!"); err == nil {
		t.Fatal("invalid QR accepted")
	}
	urlQR, err := s.Subs().QRPNG(ctx, "https://example.com/token")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Subs().ImportFromQR(ctx, urlQR); err == nil {
		t.Fatal("subscription URL imported without source confirmation")
	}
}

func TestBackupCancellationAndShareEmptyState(t *testing.T) {
	s, _ := emptySession()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := s.System().ExportBackup(ctx, "unused"); err == nil {
		t.Fatal("cancelled backup accepted")
	}
	if err := s.System().RestoreBackup(ctx, "unused"); err == nil {
		t.Fatal("cancelled restore accepted")
	}
	if err := s.System().ExportDiagnostics(ctx, "unused"); err == nil {
		t.Fatal("cancelled diagnostics accepted")
	}
	if _, err := s.Subs().LanShareStart(context.Background()); err == nil {
		t.Fatal("empty share opened listener")
	}
	if s.Subs().LanShareStatus(ctx).Active {
		t.Fatal("empty share active")
	}
	if err := s.Subs().LanShareStop(ctx); err != nil {
		t.Fatal(err)
	}
	if err := s.CloseShares(); err != nil {
		t.Fatal(err)
	}
}
