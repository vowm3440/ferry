package rulesets

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"ferry/internal/spec"
	"github.com/sagernet/sing-box/option"
)

// Manager owns an offline cache. Updates are serialized and cancellable;
// readers keep using the previous complete file until atomic publication.
type Manager struct {
	root         string
	client       *http.Client
	customClient *http.Client
	gate         chan struct{}
	mu           sync.RWMutex
	status       map[string]string
	custom       []Info
}

// New accepts a trusted, absolute data directory, such as the user-mode core's
// %LOCALAPPDATA%/Ferry/rulesets. Construction performs no filesystem or network IO.
func New(root string) *Manager { return NewWithClient(root, nil) }

// NewWithClient uses only the supplied client's transport and bounded timeout.
func NewWithClient(root string, client *http.Client) *Manager {
	return &Manager{
		root:         root,
		client:       downloadClient(client, checkRedirect),
		customClient: downloadClient(client, checkCustomRedirect),
		gate:         make(chan struct{}, 1),
		status:       make(map[string]string),
	}
}

// SyncCustom replaces the in-memory custom remote rule-set definitions.
func (m *Manager) SyncCustom(sets []spec.RuleSet) error {
	if len(sets) > spec.MaxCustomRuleSets {
		return errors.New("自定义规则集数量超过限制")
	}
	next := make([]Info, 0, len(sets))
	seen := make(map[string]bool, len(sets))
	for _, rs := range sets {
		if seen[rs.ID] {
			return errors.New("自定义规则集标识重复")
		}
		if err := spec.ValidateRuleSet(rs); err != nil {
			return err
		}
		seen[rs.ID] = true
		next = append(next, Info{
			ID: rs.ID, Name: rs.Name, URL: rs.URL, Status: "missing",
			Custom: true, AutoUpdate: rs.AutoUpdate, UpdateHours: rs.UpdateHours,
		})
	}
	m.mu.Lock()
	m.custom = next
	m.mu.Unlock()
	return nil
}

// Delete removes a custom rule-set's cached file (ignoring missing files).
func (m *Manager) Delete(id string) error {
	if !spec.ValidID(id) || spec.ValidRuleSetID(id) {
		return errors.New("不能删除内置或无效的规则集")
	}
	m.mu.Lock()
	delete(m.status, id)
	m.mu.Unlock()
	root, err := openCacheRoot(m.root, false)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	defer root.Close()
	if err := root.Remove(id + ".srs"); err != nil && !errors.Is(err, os.ErrNotExist) {
		return errors.New("cannot remove rule-set cache")
	}
	return nil
}

func (m *Manager) definition(id string) (Info, error) {
	if item, err := builtinDefinition(id); err == nil {
		return item, nil
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, item := range m.custom {
		if item.ID == id {
			return item, nil
		}
	}
	return Info{}, errors.New("unknown rule-set ID")
}

// List validates local files only. A failed refresh reports error while retaining
// the previous valid file's metadata, so callers can distinguish usable fallback.
func (m *Manager) List() []Info {
	result := Catalog()
	m.mu.RLock()
	result = append(result, m.custom...)
	m.mu.RUnlock()
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

// Resolve verifies a local resource and returns its core-owned path without downloading.
func (m *Manager) Resolve(id string) (string, error) {
	if _, _, err := m.load(id); err != nil {
		return "", err
	}
	return filepath.Join(m.root, id+".srs"), nil
}

// Load returns a validated, independent in-memory rule-set snapshot.
func (m *Manager) Load(id string) (option.PlainRuleSet, error) {
	rules, _, err := m.load(id)
	return rules, err
}

func (m *Manager) load(id string) (option.PlainRuleSet, os.FileInfo, error) {
	if _, err := m.definition(id); err != nil {
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

// Update fetches a built-in or synced custom rule-set URL.
func (m *Manager) Update(ctx context.Context, id string) (err error) {
	item, err := m.definition(id)
	if err != nil || ctx.Err() != nil {
		if err != nil {
			return err
		}
		return ctx.Err()
	}
	select {
	case m.gate <- struct{}{}:
		defer func() { <-m.gate }()
	case <-ctx.Done():
		return ctx.Err()
	}
	m.setStatus(id, "updating")
	defer func() { m.finishUpdate(id, err) }()
	return m.downloadAndPublish(ctx, item)
}

func (m *Manager) finishUpdate(id string, err error) {
	state := ""
	if err != nil {
		state = "error"
	}
	m.setStatus(id, state)
}

func (m *Manager) downloadAndPublish(ctx context.Context, item Info) error {
	root, err := openCacheRoot(m.root, true)
	if err != nil {
		return err
	}
	defer root.Close()
	if err = safeDestination(root, item.ID+".srs"); err != nil {
		return err
	}
	data, err := m.fetch(ctx, item.URL, item.Custom)
	if err != nil {
		return err
	}
	if _, err = validateSRSContext(ctx, data); err != nil {
		return err
	}
	return publish(ctx, root, item.ID+".srs", data)
}

func (m *Manager) setStatus(id, state string) {
	m.mu.Lock()
	m.status[id] = state
	m.mu.Unlock()
}
