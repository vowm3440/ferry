//go:build windows

package ipc

import (
	"context"
	"errors"
	"net"
	"regexp"

	"github.com/Microsoft/go-winio"
	"golang.org/x/sys/windows"
)

var validPipe = regexp.MustCompile(`^\\\\\.\\pipe\\ferry-core-[0-9a-f]{32}$`)

func Listen(name string) (net.Listener, error) {
	if !validPipe.MatchString(name) {
		return nil, errors.New("管道名称无效")
	}
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return nil, errors.New("无法读取当前用户身份")
	}
	// Protected DACL: this user's logon is the only allowed principal.
	// go-winio always includes FILE_PIPE_REJECT_REMOTE_CLIENTS.
	security := "D:P(A;;GA;;;" + user.User.Sid.String() + ")"
	return winio.ListenPipe(name, &winio.PipeConfig{SecurityDescriptor: security, InputBufferSize: 65536, OutputBufferSize: 65536})
}

func dialPipe(ctx context.Context, name string) (net.Conn, error) {
	if !validPipe.MatchString(name) {
		return nil, errors.New("管道名称无效")
	}
	return winio.DialPipeContext(ctx, name)
}
