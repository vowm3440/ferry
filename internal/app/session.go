// Package app exposes UI contracts without importing the privileged core.
package app

import (
	"context"
	"encoding/json"
	"sync"

	"ferry/internal/ipc"
	"ferry/internal/release"
	"ferry/internal/spec"
)

type Remote interface {
	Status(context.Context) (ipc.State, error)
	Start(context.Context, json.RawMessage) error
	Stop(context.Context) error
	SetMode(context.Context, string) error
	SelectNode(context.Context, string, string) error
	Snapshot(context.Context) (ipc.Traffic, error)
}

type Session struct {
	mu         sync.Mutex
	remote     Remote
	config     spec.RunSpec
	settings   SettingsValue
	favorites  map[string]bool
	selected   map[string]string
	subs       []subscription
	storage    Storage
	revision   uint64
	emitMu     sync.RWMutex
	emitter    func(string, any)
	inspection inspectionState
	proxy           ProxyController
	serviceCtrl     ServiceController
	lan             lanState
	recovery        recoveryState
	autoStart       AutoStartController
	configDir       string
	logDir          string
	openDir         DirectoryOpener
	lightweightHook  func(bool) error
	languageHook     func(string)
	screenCapture    func() (string, error)
	deepLinkReg      DeepLinkRegistrar
	serviceRestarter func(startCore, restoreProxy bool) error
	updater          updateRuntime
	lastRuleSetSync  int64
}

func NewSession(remote Remote) *Session {
	return &Session{remote: remote, config: spec.Default(), settings: defaultSettings(), favorites: map[string]bool{}, selected: map[string]string{}}
}

func (s *Session) Services() (*Core, *Nodes, *Settings) { return &Core{s}, &Nodes{s}, &Settings{s} }

func (s *Session) state(ctx context.Context) ipc.State {
	state, err := s.remote.Status(ctx)
	if err != nil {
		state = ipc.State{Status: "error", Error: "内核连接已断开，请重新启动"}
	}
	state.Mode = s.config.Mode
	state.Version = release.Version
	if state.CoreVersion == "" {
		state.CoreVersion = "1.14.4"
	}
	return state
}
