// Package spec defines the restricted, version-independent core input.
// It deliberately imports no sing-box packages so the GUI stays lightweight.
package spec

import "encoding/json"

const (
	MaxBytes            = 1 << 20
	MaxNodes            = 1024
	MaxGroups           = 128
	MaxRules            = 2048
	MaxRuleItems        = 256
	MaxAppGroups        = 128
	MaxExpandedRuleApps = 1024
	MaxTotalRuleApps    = 16384
	MaxCustomRuleSets   = 64
	MaxDNSServers       = 32
	MaxDNSRules         = 128
)

type RunSpec struct {
	Nodes     []Node     `json:"nodes"`
	Groups    []Group    `json:"groups"`
	AppGroups []AppGroup `json:"appGroups,omitempty"`
	RuleSets  []RuleSet  `json:"ruleSets,omitempty"`
	Rules     []Rule     `json:"rules"`
	DNS       DNS        `json:"dns"`
	Inbound   Inbound    `json:"inbound"`
	Mode      string     `json:"mode"`
	Log       Log        `json:"log"`
}

type RuleSet struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	URL         string `json:"url"`
	AutoUpdate  bool   `json:"autoUpdate,omitempty"`
	UpdateHours int    `json:"updateHours,omitempty"`
}

type Node struct {
	ID       string          `json:"id"`
	Tag      string          `json:"tag"`
	Outbound json.RawMessage `json:"outbound"`
}

type Group struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Type     string   `json:"type"`
	Members  []string `json:"members"`
	TestURL  string   `json:"testUrl"`
	Interval string   `json:"interval"`
}

type Rule struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Enabled       bool     `json:"enabled"`
	Apps          []string `json:"apps"`
	Hosts         []string `json:"hosts"`
	Ports         []string `json:"ports"`
	Network       string   `json:"network"`
	Action        string   `json:"action"`
	TargetID      string   `json:"targetId,omitempty"`
	ResolveBefore bool     `json:"resolveBefore"`
	Pinned        bool     `json:"pinned,omitempty"`
}

type AppGroup struct {
	ID   string   `json:"id"`
	Name string   `json:"name"`
	Apps []string `json:"apps"`
}

type DNSServer struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Address string `json:"address"`
	Detour  string `json:"detour,omitempty"`
}

type DNSRule struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Enabled bool     `json:"enabled"`
	Domains []string `json:"domains"`
	Server  string   `json:"server"`
}

type DNS struct {
	Mode     string      `json:"mode"`
	Remote   string      `json:"remote"`
	Direct   string      `json:"direct"`
	Strategy string      `json:"strategy"`
	Servers  []DNSServer `json:"servers,omitempty"`
	Rules    []DNSRule   `json:"rules,omitempty"`
}

type Inbound struct {
	MixedPort uint16 `json:"mixedPort"`
	AllowLAN  bool   `json:"allowLan"`
	Tun       Tun    `json:"tun"`
}

type Tun struct {
	Enabled     bool     `json:"enabled"`
	Stack       string   `json:"stack"`
	StrictRoute bool     `json:"strictRoute"`
	Excludes    []string `json:"excludes"`
	MTU         uint32   `json:"mtu"`
}

type Log struct {
	Level string `json:"level"`
}
