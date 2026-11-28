package host

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"strconv"
	"sync"
	"time"

	"ferry/internal/core/compile"
	"ferry/internal/telemetry"
)

// A cold probe pays for two handshakes (to the node, then through it to the
// test site); 2 s reported working long-haul nodes as timed out. 5 s is what
// Clash-family clients use. The HTTP client must outlast the probe itself.
const (
	latencyProbeMs      = 5000
	latencyProbeWorkers = 16 // 32 nodes per IPC batch => at most 2 rounds, well inside the 20 s IPC limit
)

var connectionID = regexp.MustCompile(`^[a-fA-F0-9-]{36}$`)

func (h *Host) LatencyTest(ctx context.Context, ids []string) (map[string]int, error) {
	i := h.active()
	if i == nil {
		return nil, errStopped
	}
	if len(ids) == 0 || len(ids) > 128 {
		return nil, errors.New("每批测速须选择 1 到 128 个节点")
	}
	for _, id := range ids {
		if !i.nodes[id] {
			return nil, errors.New("测速节点不存在")
		}
	}
	result := make(map[string]int, len(ids))
	var mu sync.Mutex
	var workers sync.WaitGroup
	slots := make(chan struct{}, latencyProbeWorkers)
	prober := &http.Client{Timeout: (latencyProbeMs + 2000) * time.Millisecond, Transport: i.client.Transport}
	for _, id := range ids {
		select {
		case slots <- struct{}{}:
		case <-ctx.Done():
			workers.Wait()
			return result, errCanceled
		}
		workers.Add(1)
		go func(id string) {
			defer workers.Done()
			defer func() { <-slots }()
			var value struct {
				Delay int `json:"delay"`
			}
			path := "/proxies/" + url.PathEscape(id) + "/delay?timeout=" + strconv.Itoa(latencyProbeMs) + "&url=" + url.QueryEscape("https://www.gstatic.com/generate_204")
			delay := -1
			if err := i.requestWith(prober, ctx, http.MethodGet, path, nil, &value); err == nil && value.Delay >= 0 {
				delay = value.Delay
			}
			mu.Lock()
			result[id] = delay
			mu.Unlock()
		}(id)
	}
	workers.Wait()
	if ctx.Err() != nil {
		return result, errCanceled
	}
	return result, nil
}

type rawConnection struct {
	ID       string `json:"id"`
	Metadata struct {
		Network         string `json:"network"`
		Host            string `json:"host"`
		DestinationIP   string `json:"destinationIP"`
		DestinationPort string `json:"destinationPort"`
		ProcessPath     string `json:"processPath"`
	} `json:"metadata"`
	Upload   int64     `json:"upload"`
	Download int64     `json:"download"`
	Start    time.Time `json:"start"`
	Chains   []string  `json:"chains"`
	Rule     string    `json:"rule"`
}

func (h *Host) Connections(ctx context.Context) ([]telemetry.Connection, error) {
	result := []telemetry.Connection{}
	i := h.active()
	if i == nil {
		return result, nil
	}
	var response struct {
		Connections []rawConnection `json:"connections"`
	}
	if err := i.request(ctx, http.MethodGet, "/connections", nil, &response); err != nil {
		return nil, err
	}
	if len(response.Connections) > 10000 {
		return nil, errors.New("连接数量超过显示限制")
	}
	for _, c := range response.Connections {
		item, err := mapRawConnection(c, i.ruleLookup)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, nil
}

func mapRawConnection(c rawConnection, lookup map[string]compile.RuleMeta) (telemetry.Connection, error) {
	port, err := strconv.Atoi(c.Metadata.DestinationPort)
	if err != nil || port < 0 || port > 65535 {
		return telemetry.Connection{}, errors.New("连接端口无效")
	}
	item := telemetry.Connection{
		ID: c.ID, ProcessPath: c.Metadata.ProcessPath, Host: c.Metadata.Host, Port: port,
		Network: c.Metadata.Network, RuleName: c.Rule, Action: "proxy",
		Chain: c.Chains, Up: c.Upload, Down: c.Download, StartAt: c.Start.UnixMilli(),
	}
	if attr, ok := lookup[c.Rule]; ok {
		item.RuleID, item.RuleName, item.RuleDetail = attr.RuleID, attr.RuleName, c.Rule
	}
	if item.ProcessPath != "" {
		item.Process = filepath.Base(item.ProcessPath)
	}
	if item.Host == "" {
		item.Host = c.Metadata.DestinationIP
	}
	if item.Chain == nil {
		item.Chain = []string{}
	}
	if len(c.Chains) > 0 {
		item.OutboundTag = c.Chains[0]
	}
	if item.OutboundTag == "__direct" || item.OutboundTag == "DIRECT" {
		item.Action = "direct"
	}
	if item.OutboundTag == "REJECT" || item.OutboundTag == "__block" {
		item.Action = "block"
	}
	return item, nil
}

func (h *Host) CloseConnection(ctx context.Context, id string) error {
	i := h.active()
	if i == nil {
		return errStopped
	}
	path := "/connections"
	if id != "" {
		if !connectionID.MatchString(id) {
			return errors.New("连接标识无效")
		}
		path += "/" + id
	}
	return i.request(ctx, http.MethodDelete, path, nil, nil)
}
