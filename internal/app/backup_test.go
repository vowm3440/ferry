package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBackupEncryptedRoundTripAndDiagnosticsRedaction(t *testing.T) {
	s, _ := emptySession()
	ctx := context.Background()
	if _, err := s.Subs().AddFromText(ctx, "private-name", "socks5://private-user:backup-secret@192.168.50.10:1080"); err != nil {
		t.Fatal(err)
	}
	s.subs[0].url = "https://private.example/access-token"
	path := filepath.Join(t.TempDir(), "backup.ferry")
	if err := s.System().ExportBackup(ctx, path); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"backup-secret", "private-user", "access-token", "192.168.50.10"} {
		if strings.Contains(string(data), secret) {
			t.Fatal("plaintext backup secret")
		}
	}
	restored, _ := emptySession()
	if err = restored.System().RestoreBackup(ctx, path); err != nil {
		t.Fatal(err)
	}
	if len(restored.config.Nodes) != 1 || restored.subs[0].url != s.subs[0].url {
		t.Fatal("backup did not restore complete configuration")
	}
	if err = s.System().ExportBackup(ctx, path); err == nil {
		t.Fatal("backup overwrote existing file")
	}
	diagnostics := filepath.Join(t.TempDir(), "diagnostics.json")
	if err = s.System().ExportDiagnostics(ctx, diagnostics); err != nil {
		t.Fatal(err)
	}
	data, err = os.ReadFile(diagnostics)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"backup-secret", "private-user", "access-token", "192.168.50.10", "private-name", "private.example"} {
		if strings.Contains(string(data), secret) {
			t.Fatal("diagnostics exposed identifying configuration")
		}
	}
}

func TestInvalidBackupDoesNotReplaceCurrentState(t *testing.T) {
	s, _ := configured(t)
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "bad.ferry")
	if err := os.WriteFile(path, []byte("invalid"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := s.System().RestoreBackup(ctx, path); err == nil {
		t.Fatal("corrupt backup accepted")
	}
	if len(s.config.Nodes) != 1 || s.config.Nodes[0].ID != "test" {
		t.Fatal("failed restore changed nodes")
	}
}
