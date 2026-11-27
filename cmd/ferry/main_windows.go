//go:build windows

package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"ferry/internal/app"
	"ferry/internal/instance"
	"ferry/internal/ipc"
	"ferry/internal/ui"

	"github.com/wailsapp/wails/v3/pkg/application"
)

type launchOptions struct {
	deepLink            string
	afterServiceInstall bool
	autoStartCore       bool
	restoreSysProxy     bool
}

func main() {
	opts := parseLaunchOptions(os.Args[1:])
	lock, err := acquireInstanceLock(opts.afterServiceInstall)
	if err != nil {
		if handleRunningInstance(err, opts.deepLink) {
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
	runDesktop(session, router, lock, executable, opts)
}

func handleRunningInstance(err error, deepLink string) bool {
	if !errors.Is(err, instance.ErrRunning) {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if deepLink != "" {
		return instance.ForwardDeepLink(ctx, deepLink) == nil
	}
	return instance.ForwardActivate(ctx) == nil
}

func acquireInstanceLock(waitForHandoff bool) (*instance.Lock, error) {
	if waitForHandoff {
		return instance.AcquireWithTimeout(10 * time.Second)
	}
	return instance.Acquire()
}

func runDesktop(session *app.Session, router *ipc.Router, lock *instance.Lock, executable string, opts launchOptions) {
	core, nodes, settings := session.Services()
	ctx, cancel := context.WithCancel(context.Background())
	var stopOnce sync.Once
	shutdownAll := func() { stopOnce.Do(func() { shutdown(cancel, session, router) }) }
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
		OnShutdown: shutdownAll,
	})
	session.SetEmitter(func(name string, value any) { desktop.Event.Emit(name, value) })
	go session.RunBackground(ctx)
	cfg, _ := settings.Get(ctx)
	startMinimized := cfg.StartMinimized && !opts.afterServiceInstall
	winCtrl := newDesktopWindowController(desktop, startMinimized, cfg.Lightweight)
	session.SetLightweightHook(func(on bool) error { winCtrl.SetLightweight(on); return nil })
	session.SetUpdateInstaller(nil, func() { winCtrl.MarkQuitting(); desktop.Quit() })
	session.SetServiceRestarter(makeServiceRestarter(executable, winCtrl, shutdownAll, lock.Close, desktop.Quit))
	setupTray(desktop, winCtrl, session, cfg.Language)
	startDeepLinkListener(ctx, desktop, winCtrl, opts.deepLink)
	if opts.afterServiceInstall && opts.autoStartCore {
		go autoStartPostServiceInstall(ctx, desktop, core, session, opts.restoreSysProxy)
	}
	go emitSnapshots(ctx, desktop, core, session, winCtrl)
	if err := desktop.Run(); err != nil {
		shutdownAll()
		slog.Error("无法启动 Ferry 窗口")
	}
}

func makeServiceRestarter(executable string, winCtrl *windowController, shutdownAll func(), releaseLock func() error, quit func()) func(bool, bool) error {
	return func(startCore, restoreProxy bool) error {
		args := []string{"--after-service-install"}
		if startCore {
			args = append(args, "--auto-start-core")
		}
		if restoreProxy {
			args = append(args, "--restore-sysproxy")
		}
		go func() {
			time.Sleep(350 * time.Millisecond)
			winCtrl.MarkQuitting()
			shutdownAll()
			_ = releaseLock()
			_ = exec.Command(executable, args...).Start()
			quit()
			time.AfterFunc(3*time.Second, func() { os.Exit(0) })
		}()
		return nil
	}
}

func autoStartPostServiceInstall(ctx context.Context, desktop *application.App, core *app.Core, session *app.Session, restoreProxy bool) {
	startCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	_ = session.AutoStartAfterServiceInstall(startCtx, restoreProxy)
	statusCtx, statusCancel := context.WithTimeout(ctx, 2*time.Second)
	defer statusCancel()
	desktop.Event.Emit("core:state", core.Status(statusCtx))
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
		statusCtx, statusCancel := context.WithTimeout(ctx, 2*time.Second)
		state := core.Status(statusCtx)
		statusCancel()
		if recoveryErr != nil {
			state.Error = recoveryErr.Error()
		}
		if state.Status != previous.Status || state.Mode != previous.Mode || state.Error != previous.Error || state.ServiceMode != previous.ServiceMode {
			desktop.Event.Emit("core:state", state)
		}
		if !isHidden || winCtrl == nil || !winCtrl.IsLightweight() {
			emitTelemetryStep(ctx, desktop, core, session, state.Status, previous.Status)
		}
		previous = state
	}
}

func emitTelemetryStep(ctx context.Context, desktop *application.App, core *app.Core, session *app.Session, currentStatus, previousStatus string) {
	pollCtx, pollCancel := context.WithTimeout(ctx, 2*time.Second)
	defer pollCancel()
	if currentStatus == "running" {
		if traffic, err := core.Snapshot(pollCtx); err == nil {
			desktop.Event.Emit("traffic", traffic)
		}
		session.PollTelemetry(pollCtx)
		return
	}
	if previousStatus == "running" {
		session.ResetTelemetryOnStop(pollCtx)
	}
}

func parseLaunchOptions(args []string) launchOptions {
	var opts launchOptions
	for _, arg := range args {
		trimmed := strings.TrimSpace(arg)
		lower := strings.ToLower(trimmed)
		switch {
		case trimmed == "--after-service-install":
			opts.afterServiceInstall = true
		case trimmed == "--auto-start-core":
			opts.autoStartCore = true
		case trimmed == "--restore-sysproxy":
			opts.restoreSysProxy = true
		case opts.deepLink == "" && (strings.HasPrefix(lower, "ferry://") || strings.HasPrefix(lower, "clash://") || strings.HasPrefix(lower, "sing-box://")):
			opts.deepLink = trimmed
		}
	}
	return opts
}

func startDeepLinkListener(ctx context.Context, desktop *application.App, winCtrl *windowController, initial string) {
	deliver := func(raw string) {
		if raw == instance.ActivateToken {
			winCtrl.Show()
			return
		}
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
