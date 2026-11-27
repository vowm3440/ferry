// Package ipc is the narrow, unprivileged GUI/core control protocol.
// It deliberately has no dependency on sing-box or the core implementation.
package ipc

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
)

const MaxBodySize = 1 << 20

type State struct {
	Status      string `json:"status"`
	Mode        string `json:"mode"`
	UptimeMs    int64  `json:"uptimeMs"`
	MemoryBytes int64  `json:"memoryBytes"`
	Version     string `json:"version"`
	CoreVersion string `json:"coreVersion"`
	ServiceMode bool   `json:"serviceMode"`
	Error       string `json:"error,omitempty"`
}

type Traffic struct {
	Up          int64   `json:"up"`
	Down        int64   `json:"down"`
	ActiveConns int64   `json:"activeConns"`
	TotalUp     int64   `json:"totalUp"`
	TotalDown   int64   `json:"totalDown"`
	MemoryBytes int64   `json:"memoryBytes"`
	UpHistory   []int64 `json:"upHistory"`
	DownHistory []int64 `json:"downHistory"`
}

type Credentials struct {
	Pipe  string `json:"pipe"`
	Token string `json:"token"`
}

func NewCredentials() (Credentials, error) {
	var data [48]byte
	if _, err := rand.Read(data[:]); err != nil {
		return Credentials{}, err
	}
	return Credentials{Pipe: `\\.\pipe\ferry-core-` + hex.EncodeToString(data[:16]), Token: hex.EncodeToString(data[16:])}, nil
}

type Backend interface {
	Status() State
	Start(context.Context, json.RawMessage) error
	Stop(context.Context) error
	SetMode(context.Context, string) error
	SelectNode(context.Context, string, string) error
	Snapshot(context.Context) (Traffic, error)
}

type ModeRequest struct {
	Mode string `json:"mode"`
}
type SelectRequest struct {
	GroupID string `json:"groupId"`
	NodeID  string `json:"nodeId"`
}
type responseError struct {
	Error string `json:"error"`
}
