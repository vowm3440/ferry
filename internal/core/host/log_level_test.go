package host

import (
	"ferry/internal/spec"
	"github.com/sagernet/sing-box/log"
	"testing"
)

func TestPlatformLogCaptureHonorsConfiguredLevel(t *testing.T) {
	s := spec.Default()
	s.Log.Level = "error"
	buffer := newLogBuffer()
	buffer.configure(s)
	buffer.WriteMessage(log.LevelInfo, "not retained")
	buffer.WriteMessage(log.LevelError, "retained")
	entries := buffer.snapshot(0, 200)
	if len(entries) != 1 || entries[0].Message != "retained" {
		t.Fatalf("wrong level filtering: %+v", entries)
	}
}
