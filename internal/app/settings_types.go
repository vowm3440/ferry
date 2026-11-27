package app

type TunSettings struct {
	Stack       string   `json:"stack"`
	StrictRoute bool     `json:"strictRoute"`
	Excludes    []string `json:"excludes"`
	MTU         uint32   `json:"mtu"`
	FakeIP      bool     `json:"fakeIp"`
}
type DNSSettings struct {
	Remote   string `json:"remote"`
	Direct   string `json:"direct"`
	Strategy string `json:"strategy"`
}
type SettingsValue struct {
	AutoStart               bool        `json:"autoStart"`
	StartMinimized          bool        `json:"startMinimized"`
	Lightweight             bool        `json:"lightweight"`
	Language                string      `json:"language"`
	Theme                   string      `json:"theme"`
	MixedPort               int         `json:"mixedPort"`
	AllowLAN                bool        `json:"allowLan"`
	SysProxyOnStart         bool        `json:"sysProxyOnStart"`
	Tun                     TunSettings `json:"tun"`
	DNS                     DNSSettings `json:"dns"`
	RuleSetAutoUpdate       bool        `json:"ruleSetAutoUpdate"`
	SubscriptionUA          string      `json:"subscriptionUA"`
	SubscriptionUpdateHours int         `json:"subscriptionUpdateHours"`
	LogLevel                string      `json:"logLevel"`
	UpdateChannel           string      `json:"updateChannel"`
	ClashAPIMode            string      `json:"clashApiMode"`
}
