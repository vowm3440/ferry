package app

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"time"
)

type Node struct {
	ID            string `json:"id"`
	Tag           string `json:"tag"`
	Name          string `json:"name"`
	GroupID       string `json:"groupId"`
	Type          string `json:"type"`
	Server        string `json:"server"`
	Port          uint16 `json:"port"`
	LatencyMs     int    `json:"latencyMs"`
	LatencyStatus string `json:"latencyStatus"`
	Selected      bool   `json:"selected"`
	Favorite      bool   `json:"favorite"`
}
type Group struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Type        string   `json:"type"`
	Members     []string `json:"members"`
	Selected    string   `json:"selected,omitempty"`
	TestURL     string   `json:"testUrl"`
	IntervalMin int      `json:"intervalMin"`
}
type NodeList struct {
	Groups []Group `json:"groups"`
	Nodes  []Node  `json:"nodes"`
}
type Nodes struct{ session *Session }

func (n *Nodes) List(context.Context) (NodeList, error) {
	n.session.mu.Lock()
	defer n.session.mu.Unlock()
	list := NodeList{Groups: []Group{}, Nodes: []Node{}}
	for _, g := range n.session.config.Groups {
		selected := n.session.selected[g.ID]
		if selected == "" && len(g.Members) > 0 {
			selected = g.Members[0]
		}
		interval, _ := time.ParseDuration(g.Interval) // already validated by RunSpec.
		list.Groups = append(list.Groups, Group{ID: g.ID, Name: g.Name, Type: g.Type, Members: slices.Clone(g.Members), Selected: selected, TestURL: g.TestURL, IntervalMin: int(interval.Minutes())})
	}
	if len(list.Groups) == 0 && len(n.session.config.Nodes) > 0 {
		list.Groups = n.defaultGroup()
	}
	for _, node := range n.session.config.Nodes {
		var display struct {
			Type   string `json:"type"`
			Server string `json:"server"`
			Port   uint16 `json:"server_port"`
		}
		if json.Unmarshal(node.Outbound, &display) != nil {
			return NodeList{}, errors.New("节点数据无效")
		}
		v := Node{ID: node.ID, Tag: node.Tag, Name: node.Tag, Type: display.Type, Server: display.Server, Port: display.Port, LatencyMs: -1, LatencyStatus: "idle", Favorite: n.session.favorites[node.ID]}
		n.session.inspection.mu.Lock()
		if measured, ok := n.session.inspection.latencies[node.ID]; ok {
			v.LatencyMs, v.LatencyStatus = measured.Milliseconds, measured.Status
		}
		n.session.inspection.mu.Unlock()
		for _, group := range list.Groups {
			if slices.Contains(group.Members, node.ID) {
				v.GroupID = group.ID
				v.Selected = group.Selected == node.ID
				break
			}
		}
		list.Nodes = append(list.Nodes, v)
	}
	return list, nil
}

func (n *Nodes) defaultGroup() []Group {
	members := make([]string, 0, len(n.session.config.Nodes))
	for _, node := range n.session.config.Nodes {
		members = append(members, node.ID)
	}
	selected := n.session.selected["proxy"]
	if selected == "" {
		selected = members[0]
	}
	return []Group{{ID: "proxy", Name: "代理", Type: "select", Members: members, Selected: selected}}
}

func (n *Nodes) SetFavorite(_ context.Context, id string, on bool) error {
	n.session.mu.Lock()
	defer n.session.mu.Unlock()
	for _, node := range n.session.config.Nodes {
		if node.ID == id {
			next := n.session.snapshot()
			next.Favorites[id] = on
			return n.session.commit(next)
		}
	}
	return errors.New("节点不存在")
}
