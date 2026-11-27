package app

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

type DirectoryOpener func(path string) error

func defaultOpenDir(path string) error {
	clean := filepath.Clean(path)
	if !filepath.IsAbs(clean) {
		return errors.New("目录路径无效")
	}
	if err := os.MkdirAll(clean, 0o700); err != nil {
		return errors.New("无法准备目录")
	}
	return exec.Command("explorer.exe", clean).Start()
}

func (s *Session) SetDataDirs(configDir, logDir string, opener DirectoryOpener) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.configDir = filepath.Clean(configDir)
	s.logDir = filepath.Clean(logDir)
	s.openDir = opener
}

func (s *Session) SetLightweightHook(hook func(bool) error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lightweightHook = hook
}

func (s *Session) SetLanguageHook(hook func(string)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.languageHook = hook
}

// TestInbound probes the configured mixed port: when the core is running it
// measures local TCP handshake latency; when stopped it checks port availability.
func (s *System) TestInbound(ctx context.Context) (int, error) {
	s.session.mu.Lock()
	port := s.session.config.Inbound.MixedPort
	status := s.session.state(ctx).Status
	s.session.mu.Unlock()
	address := fmt.Sprintf("127.0.0.1:%d", port)
	if status == "running" {
		start := time.Now()
		dialer := net.Dialer{Timeout: 2 * time.Second}
		conn, err := dialer.DialContext(ctx, "tcp4", address)
		if err != nil {
			return -1, errors.New("混合入站端口不可连接，请检查内核状态")
		}
		_ = conn.Close()
		return int(time.Since(start).Milliseconds()), nil
	}
	ln, err := net.Listen("tcp4", address)
	if err != nil {
		return -1, errors.New("混合代理端口已被其他程序占用，请更换端口")
	}
	_ = ln.Close()
	return 0, nil
}

func (s *System) OpenConfigDir(context.Context) error {
	return s.openManagedDir(false)
}

func (s *System) OpenLogDir(context.Context) error {
	return s.openManagedDir(true)
}

func (s *System) openManagedDir(logs bool) error {
	s.session.mu.Lock()
	target := s.session.configDir
	if logs {
		target = s.session.logDir
	}
	opener := s.session.openDir
	s.session.mu.Unlock()
	if target == "" {
		return errors.New("当前运行模式未配置本地目录")
	}
	if opener == nil {
		opener = defaultOpenDir
	}
	if err := opener(target); err != nil {
		return errors.New("无法打开目标目录")
	}
	return nil
}

type Window struct{ session *Session }

func (s *Session) Window() *Window { return &Window{session: s} }

func (w *Window) SetLightweight(_ context.Context, on bool) error {
	w.session.mu.Lock()
	next := w.session.snapshot()
	next.Settings.Lightweight = on
	hook := w.session.lightweightHook
	err := w.session.commit(next)
	w.session.mu.Unlock()
	if err != nil {
		return err
	}
	if hook != nil {
		return hook(on)
	}
	return nil
}
