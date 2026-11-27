//go:build windows

package instance

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net"
	"strings"
	"time"

	"github.com/Microsoft/go-winio"
	"golang.org/x/sys/windows"
)

func deepLinkPipeName() (string, string, error) {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil || user == nil || user.User.Sid == nil {
		return "", "", errors.New("无法确定当前用户")
	}
	sid := user.User.Sid.String()
	return `\\.\pipe\ferry-deeplink-` + sid, sid, nil
}

// ListenDeepLinks accepts deep links forwarded from secondary Ferry launches
// belonging to the same Windows user and invokes handle for each valid URI.
func ListenDeepLinks(ctx context.Context, handle func(string)) error {
	pipe, sid, err := deepLinkPipeName()
	if err != nil {
		return err
	}
	return listenDeepLinksPipe(ctx, pipe, sid, handle)
}

func listenDeepLinksPipe(ctx context.Context, pipe, sid string, handle func(string)) error {
	ln, err := winio.ListenPipe(pipe, &winio.PipeConfig{
		SecurityDescriptor: "D:P(A;;GA;;;" + sid + ")",
		InputBufferSize:    8192,
		OutputBufferSize:   1024,
	})
	if err != nil {
		return errors.New("无法启动深链监听通道")
	}
	go func() {
		<-ctx.Done()
		_ = ln.Close()
	}()
	go acceptDeepLinks(ln, handle)
	return nil
}

func acceptDeepLinks(ln net.Listener, handle func(string)) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		go readDeepLinkConn(conn, handle)
	}
}

func readDeepLinkConn(conn net.Conn, handle func(string)) {
	defer conn.Close()
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	line, err := bufio.NewReader(io.LimitReader(conn, 8192)).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return
	}
	trimmed := strings.TrimSpace(line)
	if trimmed != "" && handle != nil {
		handle(trimmed)
	}
}

// ForwardDeepLink sends a single deep link to the already-running Ferry instance.
func ForwardDeepLink(ctx context.Context, raw string) error {
	pipe, _, err := deepLinkPipeName()
	if err != nil {
		return err
	}
	return forwardDeepLinkPipe(ctx, pipe, raw)
}

func forwardDeepLinkPipe(ctx context.Context, pipe, raw string) error {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || len(trimmed) > 8192 || strings.ContainsAny(trimmed, "\r\n") {
		return errors.New("深链格式无效")
	}
	conn, err := winio.DialPipeContext(ctx, pipe)
	if err != nil {
		return errors.New("无法连接已运行的 Ferry 实例")
	}
	defer conn.Close()
	_ = conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
	_, err = io.WriteString(conn, trimmed+"\n")
	return err
}
