package rulesets

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"github.com/sagernet/sing-box/option"
)

// Manager owns an offline cache. Updates are serialized and cancellable;
// readers keep using the previous complete file until atomic publication.
type Manager struct {
	root   string
	client *http.Client
	gate   chan struct{}
	mu     sync.RWMutex
	status map[string]string
}

// New accepts a trusted, absolute data directory, such as the user-mode core's
// %LOCALAPPDATA%/Ferry/rulesets. Construction performs no filesystem or network IO.
func New(root string) *Manager { return NewWithClient(root, nil) }

// NewWithClient uses only the supplied client's transport and bounded timeout.
// The transport is trusted Go configuration; cookies and redirect hooks are not
// copied. A nil client uses the default HTTPS transport.
func NewWithClient(root string, client *http.Client) *Manager {
	return &Manager{root: root, client: downloadClient(client), gate: make(chan struct{}, 1), status: make(map[string]string)}
}

// List validates local files only. A failed refresh reports error while retaining
// the previous valid file's metadata, so callers can distinguish usable fallback.
func (m *Manager) List() []Info {
	result := Catalog()
	for index := range result {
		item := &result[index]
		_, info, err := m.load(item.ID)
		switch {
		case err == nil:
			item.Status = "ready"
			item.SizeBytes, item.UpdatedAt = info.Size(), info.ModTime().UnixMilli()
		case errors.Is(err, os.ErrNotExist):
			item.Status = "missing"
		default:
			item.Status = "invalid"
		}
		m.mu.RLock()
		if state := m.status[item.ID]; state != "" {
			item.Status = state
		}
		m.mu.RUnlock()
	}
	return result
}

// Resolve verifies a local resource and returns its core-owned path without
// downloading. Prefer Load for running cores to avoid later filesystem changes.
func (m *Manager) Resolve(id string) (string, error) {
	if _, _, err := m.load(id); err != nil {
		return "", err
	}
	return filepath.Join(m.root, id+".srs"), nil
}

// Load returns a validated, independent in-memory rule-set snapshot. Matchers are
// preserved for inline sing-box rules. Updating the cache cannot change snapshots
// already used by a running core; a subsequent core start loads the new version.
func (m *Manager) Load(id string) (option.PlainRuleSet, error) {
	rules, _, err := m.load(id)
	return rules, err
}

func (m *Manager) load(id string) (option.PlainRuleSet, os.FileInfo, error) {
	if _, err := definition(id); err != nil {
		return option.PlainRuleSet{}, nil, err
	}
	root, err := openCacheRoot(m.root, false)
	if err != nil {
		return option.PlainRuleSet{}, nil, err
	}
	defer root.Close()
	data, info, err := readCacheFile(root, id+".srs")
	if err != nil {
		return option.PlainRuleSet{}, nil, err
	}
	rules, err := validateSRS(data)
	return rules, info, err
}

// Update fetches only a catalog URL. Failures and cancellation leave the previous
// cache untouched, and no running core is reconfigured by this operation.
func (m *Manager) Update(ctx context.Context, id string) (err error) {
	item, err := definition(id)
	if err != nil {
		return err
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	select {
	case m.gate <- struct{}{}:
		defer func() { <-m.gate }()
	case <-ctx.Done():
		return ctx.Err()
	}
	m.setStatus(id, "updating")
	defer func() {
		state := ""
		if err != nil {
			state = "error"
		}
		m.setStatus(id, state)
	}()
	root, err := openCacheRoot(m.root, true)
	if err != nil {
		return err
	}
	defer root.Close()
	if err = safeDestination(root, id+".srs"); err != nil {
		return err
	}
	data, err := m.fetch(ctx, item.URL)
	if err != nil {
		return err
	}
	if _, err = validateSRSContext(ctx, data); err != nil {
		return err
	}
	return publish(ctx, root, id+".srs", data)
}

func (m *Manager) setStatus(id, state string) {
	m.mu.Lock()
	m.status[id] = state
	m.mu.Unlock()
}
