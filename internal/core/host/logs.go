package host

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"ferry/internal/spec"
	"ferry/internal/telemetry"
	"github.com/sagernet/sing-box/log"
)

var logURL = regexp.MustCompile(`(?i)\b[a-z][a-z0-9+.-]*://[^\s]+`)

type logBuffer struct {
	mu      sync.Mutex
	entries []telemetry.LogEntry
	start   int
	seq     int64
	secrets []string
	level   log.Level
}

func newLogBuffer() *logBuffer { return &logBuffer{level: log.LevelInfo} }

func (b *logBuffer) configure(s spec.RunSpec) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.secrets = nil
	if level, err := log.ParseLevel(s.Log.Level); err == nil {
		b.level = level
	}
	for _, node := range s.Nodes {
		var data map[string]any
		if json.Unmarshal(node.Outbound, &data) == nil {
			b.collectSecrets(data)
		}
	}
}

func (b *logBuffer) collectSecrets(data map[string]any) {
	for key, value := range data {
		if nested, ok := value.(map[string]any); ok {
			b.collectSecrets(nested)
			continue
		}
		switch key {
		case "password", "uuid", "private_key", "pre_shared_key", "preshared_key", "auth_str", "token":
			if secret, ok := value.(string); ok && secret != "" {
				b.secrets = append(b.secrets, secret)
			}
		}
	}
}

func (b *logBuffer) WriteMessage(level log.Level, message string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if level > b.level {
		return
	}
	message = logURL.ReplaceAllString(message, "[地址已隐藏]")
	for _, secret := range b.secrets {
		message = strings.ReplaceAll(message, secret, "[凭据已隐藏]")
	}
	if len(message) > 1024 {
		message = message[:1024]
		for !utf8.ValidString(message) {
			message = message[:len(message)-1]
		}
	}
	b.seq++
	entry := telemetry.LogEntry{Seq: b.seq, Time: time.Now().UnixMilli(), Level: log.FormatLevel(level), Message: message}
	if len(b.entries) < 5000 {
		b.entries = append(b.entries, entry)
	} else {
		b.entries[b.start] = entry
		b.start = (b.start + 1) % 5000
	}
}

func (b *logBuffer) snapshot(after int64, limit int) []telemetry.LogEntry {
	b.mu.Lock()
	defer b.mu.Unlock()
	result := []telemetry.LogEntry{}
	limit = min(max(limit, 1), 200)
	for j := 0; j < len(b.entries) && len(result) < limit; j++ {
		entry := b.entries[(b.start+j)%len(b.entries)]
		if entry.Seq > after {
			result = append(result, entry)
		}
	}
	return result
}

func (h *Host) Logs(ctx context.Context, after int64, limit int) ([]telemetry.LogEntry, error) {
	if ctx.Err() != nil {
		return nil, errCanceled
	}
	if after < 0 || limit < 1 || limit > 200 {
		return nil, errors.New("日志查询范围无效")
	}
	return h.logs.snapshot(after, limit), nil
}

func (h *Host) SetLogLevel(ctx context.Context, level string) error {
	parsed, err := log.ParseLevel(level)
	if err != nil {
		return errors.New("日志级别无效")
	}
	if ctx.Err() != nil {
		return errCanceled
	}
	i := h.active()
	if i == nil {
		return errStopped
	}
	i.box.LogFactory().SetLevel(parsed)
	h.logs.mu.Lock()
	h.logs.level = parsed
	h.logs.mu.Unlock()
	return nil
}
