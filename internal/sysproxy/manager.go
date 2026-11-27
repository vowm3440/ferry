// Package sysproxy owns and restores only the WinINet changes made by Ferry.
package sysproxy

import (
	"encoding/json"
	"errors"
	"strconv"
	"sync"
)

type Config struct {
	Enabled uint32 `json:"enabled"`
	Server  string `json:"server"`
	Bypass  string `json:"bypass"`
	PAC     string `json:"pac"`
	Present uint8  `json:"present"`
}
type Registry interface {
	Read() (Config, error)
	Write(Config) error
}
type Journal interface {
	Load() ([]byte, error)
	Save([]byte) error
}
type record struct {
	Version  int    `json:"version"`
	Active   bool   `json:"active"`
	Original Config `json:"original"`
	Applied  Config `json:"applied"`
}
type Manager struct {
	mu       sync.Mutex
	registry Registry
	journal  Journal
	state    record
}

func New(registry Registry, journal Journal) *Manager {
	return &Manager{registry: registry, journal: journal, state: record{Version: 1}}
}

func (m *Manager) Enable(port int) error {
	if port < 1 || port > 65535 {
		return errors.New("系统代理端口无效")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	current, err := m.registry.Read()
	if err != nil {
		return errors.New("无法读取原系统代理设置")
	}
	original := current
	if m.state.Active && current == m.state.Applied {
		original = m.state.Original
	}
	next := record{Version: 1, Active: true, Original: original, Applied: Config{Enabled: 1, Server: "127.0.0.1:" + strconv.Itoa(port), Bypass: "<local>;localhost;127.*;[::1]", Present: 7}}
	if err := m.save(next); err != nil {
		return err
	}
	if err := m.registry.Write(next.Applied); err != nil {
		if rollback := m.registry.Write(current); rollback != nil {
			return errors.New("系统代理设置失败且回退失败，请检查系统代理设置")
		}
		if rollback := m.save(m.state); rollback != nil {
			return rollback
		}
		return errors.New("无法修改系统代理，已恢复原值")
	}
	m.state = next
	return nil
}

func (m *Manager) Disable() error { m.mu.Lock(); defer m.mu.Unlock(); return m.disable() }
func (m *Manager) disable() error {
	if !m.state.Active {
		return nil
	}
	current, err := m.registry.Read()
	if err != nil {
		return errors.New("无法读取系统代理设置")
	}
	restored := restoreOwned(current, m.state)
	if restored != current {
		if err := m.registry.Write(restored); err != nil {
			return errors.New("无法恢复原系统代理设置")
		}
	}
	next := m.state
	next.Active = false
	if err := m.save(next); err != nil {
		return err
	}
	m.state = next
	return nil
}

// Registry writes are not atomic. Restore each value still equal to the value
// we intended to write, including its presence bit, preserving foreign edits.
func restoreOwned(current Config, state record) Config {
	// ProxyEnable belongs to ProxyServer. If another client replaced the
	// endpoint, preserve the whole group instead of disabling its new proxy.
	if (current.Server != state.Applied.Server || current.Present&2 != state.Applied.Present&2) && (current.Server != state.Original.Server || current.Present&2 != state.Original.Present&2) {
		return current
	}
	result := current
	if current.Present&1 == state.Applied.Present&1 && current.Enabled == state.Applied.Enabled {
		result.Enabled = state.Original.Enabled
		result.Present = result.Present&^1 | state.Original.Present&1
	}
	for _, field := range []struct {
		bit                        uint8
		current, applied, original string
		destination                *string
	}{
		{2, current.Server, state.Applied.Server, state.Original.Server, &result.Server},
		{4, current.Bypass, state.Applied.Bypass, state.Original.Bypass, &result.Bypass},
		{8, current.PAC, state.Applied.PAC, state.Original.PAC, &result.PAC},
	} {
		if current.Present&field.bit == state.Applied.Present&field.bit && field.current == field.applied {
			*field.destination = field.original
			result.Present = result.Present&^field.bit | state.Original.Present&field.bit
		}
	}
	return result
}

func (m *Manager) Recover() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	data, err := m.journal.Load()
	if err != nil {
		return errors.New("系统代理恢复记录无法读取")
	}
	if len(data) == 0 {
		return nil
	}
	var state record
	if json.Unmarshal(data, &state) != nil || state.Version != 1 {
		return errors.New("系统代理恢复记录无效")
	}
	m.state = state
	return m.disable()
}

func (m *Manager) Status() (bool, string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	current, err := m.registry.Read()
	if err != nil {
		return false, "", errors.New("无法读取系统代理设置")
	}
	return m.state.Active && current == m.state.Applied, current.Server, nil
}

func (m *Manager) save(state record) error {
	data, err := json.Marshal(state)
	if err != nil {
		return errors.New("系统代理恢复记录无法编码")
	}
	if m.journal.Save(data) != nil {
		return errors.New("无法保存系统代理恢复记录，未执行新的设置")
	}
	return nil
}
