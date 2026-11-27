package host

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
)

func (h *Host) SetMode(ctx context.Context, mode string) error {
	if mode != "rule" && mode != "global" && mode != "direct" {
		return errors.New("代理模式无效")
	}
	return h.operate(ctx, func() error {
		running := h.active()
		if running == nil {
			return errStopped
		}
		if err := running.request(ctx, http.MethodPatch, "/configs", map[string]string{"mode": mode}, nil); err != nil {
			return err
		}
		h.mu.Lock()
		h.state.Mode = mode
		h.mu.Unlock()
		return nil
	})
}

func (h *Host) SelectNode(ctx context.Context, groupID, nodeID string) error {
	return h.operate(ctx, func() error {
		running := h.active()
		if running == nil {
			return errStopped
		}
		if !running.groups[groupID][nodeID] {
			return errors.New("节点不属于该手动选择分组")
		}
		return running.request(ctx, http.MethodPut, "/proxies/"+url.PathEscape(groupID), map[string]string{"name": nodeID}, nil)
	})
}

func (h *Host) active() *instance {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.current
}

func (i *instance) request(ctx context.Context, method, path string, input, output any) error {
	return i.requestWith(i.client, ctx, method, path, input, output)
}

// requestWith lets a caller that legitimately waits longer than the default
// control timeout (latency probes) bring its own client.
func (i *instance) requestWith(client *http.Client, ctx context.Context, method, path string, input, output any) error {
	var body io.Reader
	if input != nil {
		data, err := json.Marshal(input)
		if err != nil {
			return errors.New("内核控制参数无效")
		}
		body = bytes.NewReader(data)
	}
	request, err := http.NewRequestWithContext(ctx, method, "http://"+i.address+path, body)
	if err != nil {
		return errors.New("无法创建内核控制请求")
	}
	request.Header.Set("Authorization", "Bearer "+i.secret)
	request.Header.Set("Content-Type", "application/json")
	response, err := client.Do(request)
	if err != nil {
		return errors.New("内核控制请求失败或超时")
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return errors.New("内核拒绝控制请求")
	}
	if output == nil {
		return nil
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, (4<<20)+1))
	if err != nil || len(data) > 4<<20 || json.Unmarshal(data, output) != nil {
		return errors.New("内核返回的数据无效或超过限制")
	}
	return nil
}
