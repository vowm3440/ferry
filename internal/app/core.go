package app

import (
	"context"
	"encoding/json"
	"errors"

	"ferry/internal/ipc"
	"ferry/internal/spec"
)

type Core struct{ session *Session }

func (c *Core) Status(ctx context.Context) ipc.State {
	c.session.mu.Lock()
	defer c.session.mu.Unlock()
	return c.session.state(ctx)
}

func (c *Core) Configure(ctx context.Context, specJSON string) error {
	if len(specJSON) > spec.MaxBytes {
		return errors.New("配置过大")
	}
	config, err := spec.Decode([]byte(specJSON))
	if err != nil {
		return errors.New("配置无效，请检查节点、端口与支持的字段")
	}
	c.session.mu.Lock()
	defer c.session.mu.Unlock()
	if err := c.session.requireStopped(ctx, "请先停止内核，再导入配置"); err != nil {
		return err
	}
	next := c.session.snapshot()
	next.Config = config
	next.Settings = fromSpec(next.Settings, config)
	next.Subs = []savedSubscription{} // a developer configuration replaces imported nodes.
	next.Selected = map[string]string{}
	next.Favorites = map[string]bool{}
	return c.session.commit(next)
}

func (c *Core) Start(ctx context.Context) error {
	c.session.mu.Lock()
	defer c.session.mu.Unlock()
	if err := c.start(ctx); err != nil {
		return err
	}
	c.session.recovery.desired = true
	c.session.recovery.attempts = nil
	if c.session.settings.SysProxyOnStart && c.session.proxy != nil {
		if err := c.session.proxy.Enable(int(c.session.config.Inbound.MixedPort)); err != nil {
			return err
		}
		c.session.recovery.proxyWanted = true
	}
	return nil
}

func (c *Core) start(ctx context.Context) error {
	if c.session.state(ctx).Status == "running" {
		return nil
	}
	if len(c.session.config.Nodes) == 0 {
		return errors.New("请先在「订阅」页导入节点")
	}
	data, err := json.Marshal(c.session.config)
	if err != nil {
		return errors.New("无法生成内核配置")
	}
	if err := c.session.remote.Start(ctx, data); err != nil {
		return err
	}
	c.session.selected = map[string]string{}
	return nil
}

func (c *Core) Stop(ctx context.Context) error {
	c.session.mu.Lock()
	defer c.session.mu.Unlock()
	c.session.recovery.desired = false
	c.session.recovery.proxyWanted = false
	err := c.session.remote.Stop(ctx)
	if c.session.proxy != nil {
		err = errors.Join(err, c.session.proxy.Disable())
	}
	return err
}

func (c *Core) Restart(ctx context.Context) error {
	c.session.mu.Lock()
	defer c.session.mu.Unlock()
	if err := c.session.remote.Stop(ctx); err != nil {
		return err
	}
	if err := c.start(ctx); err != nil {
		return err
	}
	c.session.recovery.desired = true
	c.session.recovery.attempts = nil
	if c.session.settings.SysProxyOnStart && c.session.proxy != nil {
		if err := c.session.proxy.Enable(int(c.session.config.Inbound.MixedPort)); err != nil {
			return err
		}
		c.session.recovery.proxyWanted = true
	}
	return nil
}

func (c *Core) SetMode(ctx context.Context, mode string) error {
	if mode != "rule" && mode != "global" && mode != "direct" {
		return errors.New("代理模式无效")
	}
	c.session.mu.Lock()
	defer c.session.mu.Unlock()
	status, err := c.session.knownStatus(ctx)
	if err != nil {
		return err
	}
	if status == "starting" {
		return errors.New("内核正在启动，请稍后重试")
	}
	running := status == "running"
	if running {
		if err := c.session.remote.SetMode(ctx, mode); err != nil {
			return err
		}
	}
	next := c.session.snapshot()
	next.Config.Mode = mode
	if err := c.session.commit(next); err != nil {
		if running {
			if rollback := c.session.remote.SetMode(ctx, c.session.config.Mode); rollback != nil {
				return errors.New("模式保存失败，内核回退失败，请停止后重试")
			}
		}
		return err
	}
	return nil
}

func (c *Core) SelectNode(ctx context.Context, groupID, nodeID string) error {
	c.session.mu.Lock()
	defer c.session.mu.Unlock()
	if c.session.state(ctx).Status != "running" {
		return errors.New("请先启动内核再切换节点")
	}
	if err := c.session.remote.SelectNode(ctx, groupID, nodeID); err != nil {
		return err
	}
	c.session.selected[groupID] = nodeID
	return nil
}

func (c *Core) Snapshot(ctx context.Context) (ipc.Traffic, error) {
	c.session.mu.Lock()
	defer c.session.mu.Unlock()
	return c.session.remote.Snapshot(ctx)
}
