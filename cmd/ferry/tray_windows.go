//go:build windows

package main

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"math"
	"time"

	"ferry/internal/app"
	"github.com/wailsapp/wails/v3/pkg/application"
)

type trayLabels struct {
	show, start, stop          string
	rule, global, direct       string
	proxyOn, proxyOff, quit    string
}

func trayLabelsFor(lang string) trayLabels {
	if lang == "en-US" {
		return trayLabels{
			show: "Show Ferry", start: "Start Core", stop: "Stop Core",
			rule: "Rule Mode", global: "Global Mode", direct: "Direct Mode",
			proxyOn: "Enable System Proxy", proxyOff: "Restore System Proxy", quit: "Quit Ferry",
		}
	}
	return trayLabels{
		show: "显示 Ferry", start: "启动内核", stop: "停止内核",
		rule: "规则模式", global: "全局模式", direct: "直连模式",
		proxyOn: "开启系统代理", proxyOff: "恢复原系统代理", quit: "退出 Ferry",
	}
}

func buildTrayMenu(desktop *application.App, winCtrl *windowController, session *app.Session, lang string) *application.Menu {
	labels := trayLabelsFor(lang)
	menu := application.NewMenu()
	menu.Add(labels.show).OnClick(func(*application.Context) { winCtrl.Show() })
	core, _, _ := session.Services()
	add := func(label string, action func(context.Context) error) {
		menu.Add(label).OnClick(func(*application.Context) { go trayAction(desktop, action) })
	}
	add(labels.start, core.Start)
	add(labels.stop, core.Stop)
	menu.AddSeparator()
	for _, mode := range []struct{ label, value string }{{labels.rule, "rule"}, {labels.global, "global"}, {labels.direct, "direct"}} {
		value := mode.value
		add(mode.label, func(ctx context.Context) error { return core.SetMode(ctx, value) })
	}
	add(labels.proxyOn, func(ctx context.Context) error { return session.System().SetSysProxy(ctx, true) })
	add(labels.proxyOff, func(ctx context.Context) error { return session.System().SetSysProxy(ctx, false) })
	menu.AddSeparator()
	menu.Add(labels.quit).OnClick(func(*application.Context) { winCtrl.MarkQuitting(); desktop.Quit() })
	return menu
}

func setupTray(desktop *application.App, winCtrl *windowController, session *app.Session, initialLang string) {
	tray := desktop.SystemTray.New()
	tray.SetIcon(trayIcon())
	tray.SetTooltip("Ferry")
	tray.OnClick(func() { winCtrl.Show() })
	tray.SetMenu(buildTrayMenu(desktop, winCtrl, session, initialLang))
	session.SetLanguageHook(func(lang string) {
		tray.SetMenu(buildTrayMenu(desktop, winCtrl, session, lang))
	})
}

func trayAction(desktop *application.App, action func(context.Context) error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := action(ctx); err != nil {
		desktop.Dialog.Error().SetTitle("Ferry").SetMessage(err.Error()).Show()
	}
}

// Rasterise the same gradient, sail and wave as frontend/public/favicon.svg.
func trayIcon() []byte {
	img := image.NewRGBA(image.Rect(0, 0, 64, 64))
	for y := 0; y < 64; y++ {
		for x := 0; x < 64; x++ {
			dx, dy := math.Max(math.Abs(float64(x)-31.5)-16, 0), math.Max(math.Abs(float64(y)-31.5)-16, 0)
			if dx*dx+dy*dy > 16*16 {
				continue
			}
			t := float64(x+y) / 126
			img.SetRGBA(x, y, color.RGBA{uint8(60 + 95*t), uint8(99 + 24*t), uint8(239 + 16*t), 255})
		}
	}
	for i := 0; i <= 200; i++ {
		t := float64(i) / 200
		paintIconPoint(img, 6+6*t, 13-9*t)
		paintIconPoint(img, 12+6*t, 4+9*t)
		for segment := 0; segment < 2; segment++ {
			x := 3 + 9*t + float64(segment)*9
			y := 17 - 6*t*(1-t)
			if segment == 1 {
				y = 17 + 6*t*(1-t)
			}
			paintIconPoint(img, x, y)
		}
	}
	var buffer bytes.Buffer
	if png.Encode(&buffer, img) != nil {
		return nil
	}
	return buffer.Bytes()
}

func paintIconPoint(img *image.RGBA, x, y float64) {
	x = (12 + (x-12)*.78) * 64 / 24
	y = (13 + (y-12)*.78) * 64 / 24
	for py := int(y) - 3; py <= int(y)+3; py++ {
		for px := int(x) - 3; px <= int(x)+3; px++ {
			dx, dy := float64(px)-x, float64(py)-y
			if dx*dx+dy*dy < 6.3 {
				img.SetRGBA(px, py, color.RGBA{255, 255, 255, 255})
			}
		}
	}
}
