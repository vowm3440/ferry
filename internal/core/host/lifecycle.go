package host

import (
	"context"
	"errors"
	"ferry/internal/core/compile"
	"ferry/internal/spec"
	"time"
)

func (h *Host) start(ctx context.Context, s spec.RunSpec) error {
	if ctx.Err() != nil {
		return errCanceled
	}
	h.mu.Lock()
	if h.current != nil {
		h.mu.Unlock()
		return nil
	}
	h.state.Status, h.state.Error = "starting", ""
	h.mu.Unlock()
	if s.Inbound.Tun.Enabled && !h.serviceMode {
		return h.fail(errors.New("用户态内核不支持 TUN，请安装并启用系统服务"))
	}
	options, err := compile.Compile(s)
	if err != nil {
		return h.fail(err)
	}
	if err := h.loadRuleSets(s, &options); err != nil {
		return h.fail(err)
	}
	if !hasClashAPI {
		return h.fail(errors.New("当前内核构建未启用 Clash API"))
	}
	running, err := newInstance(options, s, h.logs)
	if err != nil {
		if s.Inbound.Tun.Enabled {
			return h.fail(errors.New("TUN 启动失败，请检查网络适配器冲突或系统服务权限，可退回系统代理模式"))
		}
		return h.fail(err)
	}
	if ctx.Err() != nil {
		if err := running.close(); err != nil {
			return h.fail(err)
		}
		return h.fail(errCanceled)
	}
	h.mu.Lock()
	h.current, h.sampledAt = running, running.started
	h.rateUpload, h.rateDownload = 0, 0
	h.traffic = zeroTraffic()
	h.state.Status, h.state.Mode, h.state.Error = "running", s.Mode, ""
	h.mu.Unlock()
	return nil
}

func (h *Host) stop() error {
	h.mu.RLock()
	running := h.current
	h.mu.RUnlock()
	if running != nil {
		if err := running.close(); err != nil {
			h.detach()
			return h.fail(err)
		}
	}
	h.detach()
	h.mu.Lock()
	h.state.Status, h.state.Error = "stopped", ""
	h.mu.Unlock()
	return nil
}

func (h *Host) detach() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.current = nil
	h.sampledAt = time.Time{}
	h.rateUpload, h.rateDownload = 0, 0
	h.state.UptimeMs, h.state.MemoryBytes = 0, 0
	h.traffic = zeroTraffic()
}

func (h *Host) fail(err error) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.state.Status, h.state.Error = "error", err.Error()
	return err
}
