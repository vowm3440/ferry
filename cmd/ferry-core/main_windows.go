//go:build windows

package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"syscall"
	"time"

	"ferry/internal/core/host"
	"ferry/internal/ipc"
	"ferry/internal/release"
)

func main() {
	if len(os.Args) == 2 && os.Args[1] == "--version" {
		fmt.Println("Ferry core " + release.Version + " / sing-box 1.14.4")
		return
	}
	if len(os.Args) >= 2 && (os.Args[1] == "service" || os.Args[1] == "install-service" || os.Args[1] == "uninstall-service" || os.Args[1] == "repair-service" || os.Args[1] == "refresh-installed-service") {
		if err := runServiceCommand(os.Args[1], os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
		return
	}
	if len(os.Args) != 2 || os.Args[1] != "run" {
		fmt.Fprintln(os.Stderr, "用法：ferry-core.exe run（由 Ferry 启动）或 --version")
		os.Exit(2)
	}
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "内核宿主启动或关闭失败")
		os.Exit(1)
	}
}

func run() error {
	if err := requireUserProcess(); err != nil {
		return err
	}
	reader := bufio.NewReaderSize(os.Stdin, 4096)
	credentials, err := bootstrap(reader)
	if err != nil {
		return err
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	go func() { _, _ = io.Copy(io.Discard, reader); cancel() }()
	// If the GUI dies, stdin closes. A hard deadline also bounds a stuck kernel shutdown.
	finished := make(chan struct{})
	defer close(finished)
	go func() {
		select {
		case <-ctx.Done():
			select {
			case <-finished:
			case <-time.After(5 * time.Second):
				os.Exit(1)
			}
		case <-finished:
		}
	}()
	root, err := userCacheRoot()
	if err != nil {
		return err
	}
	listener, err := ipc.Listen(credentials.Pipe)
	if err != nil {
		return err
	}
	backend := &adapter{core: host.NewWithRuleSets(root)}
	serveErr := ipc.Serve(ctx, listener, ipc.NewHandler(credentials.Token, backend))
	stopCtx, stopCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer stopCancel()
	return errors.Join(serveErr, backend.Stop(stopCtx))
}

func userCacheRoot() (string, error) {
	root := os.Getenv("FERRY_CONFIG_DIR")
	if root == "" {
		base, err := os.UserCacheDir()
		if err != nil {
			return "", errors.New("无法确定规则集缓存目录")
		}
		root = filepath.Join(base, "Ferry")
	}
	if !filepath.IsAbs(root) {
		return "", errors.New("规则集缓存目录无效")
	}
	return filepath.Join(root, "rulesets"), nil
}

func bootstrap(reader *bufio.Reader) (ipc.Credentials, error) {
	line, err := reader.ReadSlice('\n')
	if err != nil {
		return ipc.Credentials{}, errors.New("启动凭据无效")
	}
	var credentials ipc.Credentials
	if json.Unmarshal(line, &credentials) != nil || !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(credentials.Token) {
		return ipc.Credentials{}, errors.New("启动凭据无效")
	}
	return credentials, nil
}
