//go:build windows

package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"ferry/internal/app"
	"ferry/internal/instance"
	"ferry/internal/ipc"
	"ferry/internal/ui"

	"github.com/wailsapp/wails/v3/pkg/application"
)

func main() {
	deepLinkArg := findDeepLinkArg(os.Args[1:])
	lock, err := instance.Acquire()
	if err != nil {
		if errors.Is(err, instance.ErrRunning) && deepLinkArg != "" {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			_ = instance.ForwardDeepLink(ctx, deepLinkArg)
			return
		}
		startupError(err)
		return
	}
	defer func() {
		if err := lock.Close(); err != nil {
			slog.Error(err.Error())
		}
	}()
	executable, err := os.Executable()
	if err != nil {
		slog.Error("无法确定应用安装位置")
		return
	}
	coreExe := filepath.Join(filepath.Dir(executable), "ferry-core.exe")
	router := ipc.NewRouter(coreExe)
	session, err := loadSession(router)
	if err != nil {
		startupError(err)
		return
	}
	session.SetServiceController(app.NewWindowsServiceController(executable, coreExe, router.StopUserProcess))
	runDesktop(session, router, deepLinkArg)
}

func runDesktop(session *app.Session, router *ipc.Router, initialDeepLink string) {
	core, nodes, settings := session.Services()
	ctx, cancel := context.WithCancel(context.Background())
	desktop := application.New(application.Options{
		Name: "Ferry", Description: "轻量 Windows 代理客户端",
		Services: []application.Service{
			application.NewService(core), application.NewService(nodes), application.NewService(settings),
			application.NewService(session.Subs()), application.NewService(session.System()),
			application.NewService(session.Connections()), application.NewService(session.Logs()),
			application.NewService(session.Rules()), application.NewService(session.AppGroups()),
			application.NewService(session.RuleSets()), application.NewService(session.Window()),
			application.NewService(session.Update()),
		},
		Assets:     application.AssetOptions{Handler: application.AssetFileServerFS(ui.Assets())},
		Windows:    devToolsOptions(),
		OnShutdown: func() { shutdown(cancel, session, router) },
	})
	session.SetEmitter(func(name string, value any) { desktop.Event.Emit(name, value) })
	go session.RunBackground(ctx)
	cfg, _ := settings.Get(ctx)
	winCtrl := newDesktopWindowController(desktop, cfg.StartMinimized, cfg.Lightweight)
	session.SetLightweightHook(func(on bool) error { winCtrl.SetLightweight(on); return nil })
	session.SetUpdateInstaller(nil, func() { winCtrl.MarkQuitting(); desktop.Quit() })
	setupTray(desktop, winCtrl, session, cfg.Language)
	startDeepLinkListener(ctx, desktop, winCtrl, initialDeepLink)
	go emitSnapshots(ctx, desktop, core, session, winCtrl)
	if err := desktop.Run(); err != nil {
		shutdown(cancel, session, router)
		slog.Error("无法启动 Ferry 窗口")
	}
}

func shutdown(cancel context.CancelFunc, session *app.Session, router *ipc.Router) {
	cancel()
	for _, close := range []func() error{session.CloseShares, session.RestoreSystemProxy, router.Close} {
		if err := close(); err != nil {
			slog.Error(err.Error())
		}
	}
}

func emitSnapshots(ctx context.Context, desktop *application.App, core *app.Core, session *app.Session, winCtrl *windowController) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	var previous ipc.State
	ticks := 0
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		ticks++
		isHidden := winCtrl != nil && winCtrl.IsHidden()
		if isHidden && ticks%5 != 0 {
			continue
		}
		recoveryCtx, recoveryCancel := context.WithTimeout(ctx, 15*time.Second)
		recoveryErr := session.RecoverCore(recoveryCtx, time.Now())
		recoveryCancel()
		callCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		state := core.Status(callCtx)
		if recoveryErr != nil {
			state.Error = recoveryErr.Error()
		}
		if state.Status != previous.Status || state.Mode != previous.Mode || state.Error != previous.Error {
			desktop.Event.Emit("core:state", state)
		}
		if !isHidden || winCtrl == nil || !winCtrl.IsLightweight() {
			if traffic, err := core.Snapshot(callCtx); err == nil {
				desktop.Event.Emit("traffic", traffic)
			}
			session.PollTelemetry(callCtx)
		}
		cancel()
		previous = state
	}
}

func findDeepLinkArg(args []string) string {
	for _, arg := range args {
		lower := strings.ToLower(strings.TrimSpace(arg))
		if strings.HasPrefix(lower, "ferry://") || strings.HasPrefix(lower, "clash://") || strings.HasPrefix(lower, "sing-box://") {
			return strings.TrimSpace(arg)
		}
	}
	return ""
}

func startDeepLinkListener(ctx context.Context, desktop *application.App, winCtrl *windowController, initial string) {
	deliver := func(raw string) {
		preview, err := app.ParseDeepLinkURL(raw)
		if err != nil {
			return
		}
		recreated := winCtrl.Show()
		if recreated {
			time.Sleep(800 * time.Millisecond)
		}
		desktop.Event.Emit("deeplink:received", preview)
	}
	_ = instance.ListenDeepLinks(ctx, deliver)
	if initial != "" {
		go func() {
			time.Sleep(1500 * time.Millisecond)
			deliver(initial)
		}()
	}
}
