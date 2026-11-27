package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"ferry/internal/convert"
	"ferry/internal/spec"
)

const (
	defaultSubName  = "手动导入"
	maxSubNameRunes = 64
)

// Subscription follows docs/plan/api.md. Skipped is only filled in the value
// returned by an import; it explains lines that were not imported.
type Subscription struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	URLMasked     string            `json:"urlMasked"`
	NodeCount     int               `json:"nodeCount"`
	GroupCount    int               `json:"groupCount"`
	UsedBytes     int64             `json:"usedBytes"`
	TotalBytes    int64             `json:"totalBytes"`
	ExpiresAt     int64             `json:"expiresAt"`
	LastUpdatedAt int64             `json:"lastUpdatedAt"`
	IntervalHours int               `json:"intervalHours"`
	AutoUpdate    bool              `json:"autoUpdate"`
	UserAgent     string            `json:"userAgent"`
	Status        string            `json:"status"`
	Error         string            `json:"error,omitempty"`
	Skipped       []convert.Skipped `json:"skipped,omitempty"`
	Warnings      []string          `json:"warnings,omitempty"`
}

// subscription is the session record: the public value plus the nodes it owns.
type subscription struct {
	view     Subscription
	nodeIDs  []string
	groupIDs []string
	url      string
	customUA string
}

type Subs struct{ session *Session }

// Subs returns the subscription service of this session.
func (s *Session) Subs() *Subs { return &Subs{s} }

func (s *Subs) List(context.Context) ([]Subscription, error) {
	s.session.mu.Lock()
	defer s.session.mu.Unlock()
	list := make([]Subscription, 0, len(s.session.subs))
	for _, sub := range s.session.subs {
		list = append(list, sub.view)
	}
	return list, nil
}

// AddFromText imports pasted share links. Nodes live for this session only.
func (s *Subs) AddFromText(ctx context.Context, name, text string) (Subscription, error) {
	name, err := subName(name)
	if err != nil {
		return Subscription{}, err
	}
	parsed, err := convert.ParseText(text)
	if err != nil {
		return Subscription{}, err
	}
	return s.add(ctx, name, parsed, "", Subscription{})
}

func (s *Subs) add(ctx context.Context, name string, parsed convert.Result, source string, view Subscription) (Subscription, error) {
	s.session.mu.Lock()
	defer s.session.mu.Unlock()
	if err := s.session.requireStopped(ctx, "请先停止内核，再导入节点"); err != nil {
		return Subscription{}, err
	}
	config := s.session.config
	if len(s.session.subs) == 0 {
		// Nodes that came from a developer configuration are replaced, not merged.
		// Preserve arrays on the wire: the core deliberately rejects JSON null.
		config.Nodes, config.Groups = []spec.Node{}, []spec.Group{}
	}
	fresh, skipped := withoutExisting(parsed, config.Nodes)
	if len(fresh) == 0 {
		return Subscription{}, noNodesError(skipped)
	}
	config.Nodes = append(slices.Clone(config.Nodes), fresh...)
	groupIDs := mergeImportedGroups(&config, parsed.Groups)
	if len(config.Nodes) > spec.MaxNodes {
		return Subscription{}, errors.New("节点数量超过限制（最多 " + strconv.Itoa(spec.MaxNodes) + " 个）")
	}
	if err := validateImportedConfig(config); err != nil {
		return Subscription{}, err
	}
	id, err := newSubID()
	if err != nil {
		return Subscription{}, err
	}
	view.ID, view.Name, view.NodeCount = id, name, len(fresh)
	view.GroupCount = len(groupIDs)
	view.LastUpdatedAt, view.Status = time.Now().UnixMilli(), "ok"
	next := s.session.snapshot()
	next.Config = config
	next.Subs = append(next.Subs, savedSubscription{View: view, NodeIDs: nodeIDs(fresh), GroupIDs: groupIDs, URL: source})
	next.Selected = map[string]string{}
	if err := s.session.commit(next); err != nil {
		return Subscription{}, err
	}
	view.Skipped = skipped
	return view, nil
}

func validateImportedConfig(config spec.RunSpec) error {
	if err := config.Validate(); err != nil {
		return errors.New("导入的节点未通过校验")
	}
	data, err := json.Marshal(config)
	if err != nil {
		return errors.New("无法生成内核配置")
	}
	if len(data) > spec.MaxBytes {
		return errors.New("导入后的总配置超过 1 MiB 限制，请减少节点数量")
	}
	return nil
}

// Remove deletes a subscription together with the nodes it brought in.
func (s *Subs) Remove(ctx context.Context, id string) error {
	s.session.mu.Lock()
	defer s.session.mu.Unlock()
	index := slices.IndexFunc(s.session.subs, func(sub subscription) bool { return sub.view.ID == id })
	if index < 0 {
		return errors.New("订阅不存在")
	}
	if err := s.session.requireStopped(ctx, "请先停止内核，再删除订阅"); err != nil {
		return err
	}
	owned := s.session.subs[index].nodeIDs
	next := s.session.snapshot()
	next.Config.Nodes = slices.DeleteFunc(slices.Clone(next.Config.Nodes), func(node spec.Node) bool { return slices.Contains(owned, node.ID) })
	ownedGroups := s.session.subs[index].groupIDs
	next.Config.Groups = slices.DeleteFunc(slices.Clone(next.Config.Groups), func(group spec.Group) bool { return slices.Contains(ownedGroups, group.ID) })
	if err := next.Config.Validate(); err != nil {
		return errors.New("其它分组或规则仍引用此订阅，请先移除对应引用")
	}
	for _, nodeID := range owned {
		delete(next.Favorites, nodeID)
	}
	next.Subs = slices.Delete(next.Subs, index, index+1)
	next.Selected = map[string]string{}
	return s.session.commit(next)
}

func subName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return defaultSubName, nil
	}
	if utf8.RuneCountInString(name) > maxSubNameRunes || !utf8.ValidString(name) || strings.ContainsFunc(name, unicode.IsControl) {
		return "", errors.New("名称无效：最多 64 个字符，且不能包含控制字符")
	}
	return name, nil
}

// withoutExisting drops nodes the session already has and records why.
func withoutExisting(parsed convert.Result, existing []spec.Node) ([]spec.Node, []convert.Skipped) {
	known := make(map[string]bool, len(existing))
	for _, node := range existing {
		known[node.ID] = true
	}
	fresh := make([]spec.Node, 0, len(parsed.Nodes))
	skipped := slices.Clone(parsed.Skipped)
	for _, node := range parsed.Nodes {
		if known[node.ID] {
			skipped = append(skipped, convert.Skipped{Reason: "节点已存在：" + node.Tag})
			continue
		}
		fresh = append(fresh, node)
	}
	return fresh, skipped
}

func noNodesError(skipped []convert.Skipped) error {
	if len(skipped) == 0 {
		return errors.New("没有可导入的节点")
	}
	first := skipped[0]
	message := "没有可导入的节点：" + first.Reason
	if first.Line > 0 {
		message = "没有可导入的节点：第 " + strconv.Itoa(first.Line) + " 行" + first.Reason
	}
	if len(skipped) > 1 {
		message += "（共 " + strconv.Itoa(len(skipped)) + " 行被跳过）"
	}
	return errors.New(message)
}

func nodeIDs(nodes []spec.Node) []string {
	ids := make([]string, 0, len(nodes))
	for _, node := range nodes {
		ids = append(ids, node.ID)
	}
	return ids
}

func newSubID() (string, error) {
	var raw [8]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", errors.New("无法生成订阅标识")
	}
	return "s-" + hex.EncodeToString(raw[:]), nil
}
