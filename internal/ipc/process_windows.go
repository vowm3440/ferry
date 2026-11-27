//go:build windows

package ipc

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

// Process owns only the sibling user-mode core. It never elevates or installs services.
type Process struct {
	mu         sync.Mutex
	executable string
	cmd        *exec.Cmd
	input      io.WriteCloser
	client     *Client
	done       chan struct{}
	logBase    int64
	logHigh    int64
}

func NewProcess(executable string) *Process { return &Process{executable: executable} }

func (p *Process) connect(ctx context.Context) error {
	if p.cmd != nil {
		select {
		case <-p.done:
			p.dispose()
		default:
			return nil
		}
	}
	if !filepath.IsAbs(p.executable) || filepath.Base(p.executable) != "ferry-core.exe" {
		return errors.New("内核程序路径无效")
	}
	credentials, err := NewCredentials()
	if err != nil {
		return errors.New("无法创建安全内核连接")
	}
	cmd := exec.Command(p.executable, "run")
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x08000000, HideWindow: true}
	input, err := cmd.StdinPipe()
	if err != nil {
		return errors.New("无法创建内核输入通道")
	}
	if err = cmd.Start(); err != nil {
		input.Close()
		return errors.New("无法启动内核，请确认 ferry-core.exe 位于程序旁")
	}
	p.cmd = cmd
	p.logBase = p.logHigh
	p.input = input
	p.client = NewClient(credentials)
	p.done = make(chan struct{})
	go func() { _ = cmd.Wait(); close(p.done) }()
	if err = json.NewEncoder(input).Encode(credentials); err != nil {
		p.terminate()
		return errors.New("内核连接初始化失败")
	}
	ready, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err = p.awaitReady(ready); err != nil {
		p.terminate()
		return err
	}
	return nil
}

func (p *Process) awaitReady(ctx context.Context) error {
	ticker := time.NewTicker(40 * time.Millisecond)
	defer ticker.Stop()
	for {
		attempt, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
		_, err := p.client.Status(attempt)
		cancel()
		if err == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return errors.New("内核启动超时")
		case <-p.done:
			return errors.New("内核启动失败")
		case <-ticker.C:
		}
	}
}

func (p *Process) dispose() {
	if p.input != nil {
		_ = p.input.Close()
	}
	if p.client != nil {
		p.client.Close()
	}
	p.input = nil
	p.client = nil
	p.cmd = nil
}

func (p *Process) terminate() {
	if p.cmd != nil {
		_ = p.cmd.Process.Kill()
		<-p.done
	}
	p.dispose()
}

func (p *Process) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cmd == nil {
		return nil
	}
	if p.input != nil {
		_ = p.input.Close()
	}
	select {
	case <-p.done:
		p.dispose()
		return nil
	case <-time.After(5 * time.Second):
		p.terminate()
		return errors.New("内核未及时退出，已结束子进程")
	}
}

func (p *Process) Status(ctx context.Context) (State, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.client == nil {
		return State{Status: "stopped", Mode: "rule"}, nil
	}
	return p.client.Status(ctx)
}

func (p *Process) Snapshot(ctx context.Context) (Traffic, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.client == nil {
		return Traffic{UpHistory: make([]int64, 60), DownHistory: make([]int64, 60)}, nil
	}
	return p.client.Snapshot(ctx)
}

func (p *Process) Start(ctx context.Context, config json.RawMessage) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := p.connect(ctx); err != nil {
		return err
	}
	return p.client.Start(ctx, config)
}

func (p *Process) Stop(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.client == nil {
		return nil
	}
	return p.client.Stop(ctx)
}

func (p *Process) SetMode(ctx context.Context, mode string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.client == nil {
		return errors.New("内核尚未启动")
	}
	return p.client.SetMode(ctx, mode)
}

func (p *Process) SelectNode(ctx context.Context, group, node string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.client == nil {
		return errors.New("内核尚未启动")
	}
	return p.client.SelectNode(ctx, group, node)
}
