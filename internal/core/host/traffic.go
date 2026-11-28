package host

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

type connectionSnapshot struct {
	UploadTotal        int64             `json:"uploadTotal"`
	DownloadTotal      int64             `json:"downloadTotal"`
	ProxyDownloadTotal int64             `json:"-"`
	HasProxyDown       bool              `json:"-"`
	Memory             int64             `json:"memory"`
	Connections        []json.RawMessage `json:"connections"`
}

func (h *Host) Snapshot(ctx context.Context) (Traffic, error) {
	if ctx.Err() != nil {
		return zeroTraffic(), errCanceled
	}
	running := h.active()
	if running == nil {
		return zeroTraffic(), nil
	}
	var snapshot connectionSnapshot
	if err := running.request(ctx, http.MethodGet, "/connections", nil, &snapshot); err != nil {
		return zeroTraffic(), err
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.current != running {
		return zeroTraffic(), nil
	}
	if running.proxyTraffic != nil {
		snapshot.ProxyDownloadTotal = running.proxyTraffic.Download()
		snapshot.HasProxyDown = true
	}
	h.updateTraffic(snapshot)
	result := h.traffic
	result.UpHistory = append([]int64(nil), result.UpHistory...)
	result.DownHistory = append([]int64(nil), result.DownHistory...)
	return result, nil
}

func (h *Host) updateTraffic(snapshot connectionSnapshot) {
	if snapshot.UploadTotal < h.totalUpload || snapshot.DownloadTotal < h.totalDownload {
		return
	}
	if snapshot.HasProxyDown && snapshot.ProxyDownloadTotal < h.traffic.TotalDown {
		return
	}
	now := time.Now()
	seconds := now.Sub(h.sampledAt).Seconds()
	if seconds >= 1 {
		h.traffic.Up = max(0, int64(float64(snapshot.UploadTotal-h.rateUpload)/seconds))
		h.traffic.Down = max(0, int64(float64(snapshot.DownloadTotal-h.rateDownload)/seconds))
		pushHistory(h.traffic.UpHistory, h.traffic.Up, int(seconds))
		pushHistory(h.traffic.DownHistory, h.traffic.Down, int(seconds))
		h.sampledAt = now
		h.rateUpload, h.rateDownload = snapshot.UploadTotal, snapshot.DownloadTotal
	}
	h.totalUpload, h.totalDownload = snapshot.UploadTotal, snapshot.DownloadTotal
	totalDown := snapshot.DownloadTotal
	if snapshot.HasProxyDown {
		totalDown = snapshot.ProxyDownloadTotal
	}
	h.traffic.TotalUp, h.traffic.TotalDown = snapshot.UploadTotal, totalDown
	h.traffic.ActiveConns, h.traffic.MemoryBytes = int64(len(snapshot.Connections)), snapshot.Memory
	h.state.MemoryBytes = snapshot.Memory
}

func pushHistory(history []int64, value int64, seconds int) {
	seconds = min(seconds, len(history))
	copy(history, history[seconds:])
	clear(history[len(history)-seconds:])
	history[len(history)-1] = value
}

func zeroTraffic() Traffic {
	return Traffic{UpHistory: make([]int64, 60), DownHistory: make([]int64, 60)}
}
