//go:build windows

package main

import "testing"

type fakeManagedWindow struct {
	shown   int
	focused int
	hidden  int
	closed  int
	onClose func() bool // returns true if close was cancelled
}

func (f *fakeManagedWindow) ShowFocus() { f.shown++; f.focused++ }
func (f *fakeManagedWindow) HideWin()   { f.hidden++ }
func (f *fakeManagedWindow) CloseWin() {
	if f.onClose != nil && f.onClose() {
		return
	}
	f.closed++
}

func TestLightweightWindowControllerLifecycle(t *testing.T) {
	created := 0
	var lastWin *fakeManagedWindow
	ctrl := newWindowController(true, true, func(attachClose func() bool) managedWindow {
		created++
		w := &fakeManagedWindow{onClose: attachClose}
		lastWin = w
		return w
	})

	// StartMinimized + Lightweight: zero WebView2 windows created at boot.
	if created != 0 || !ctrl.IsHidden() || !ctrl.IsLightweight() {
		t.Fatalf("expected deferred window creation on minimized lightweight boot: created=%d hidden=%v", created, ctrl.IsHidden())
	}

	// Tray Show creates WebView2 on demand.
	ctrl.Show()
	if created != 1 || ctrl.IsHidden() || lastWin.shown != 1 {
		t.Fatalf("expected 1 window created and shown: created=%d hidden=%v", created, ctrl.IsHidden())
	}

	// Closing in lightweight mode allows real close (destroying WebView2) instead of hiding.
	lastWin.CloseWin()
	if lastWin.closed != 1 || lastWin.hidden != 0 || !ctrl.IsHidden() {
		t.Fatalf("expected lightweight close to destroy window: closed=%d hidden=%d isHidden=%v", lastWin.closed, lastWin.hidden, ctrl.IsHidden())
	}

	// Showing again recreates a fresh WebView2 window.
	ctrl.Show()
	if created != 2 || ctrl.IsHidden() {
		t.Fatalf("expected second window created after lightweight destroy: created=%d", created)
	}

	// Turning lightweight OFF makes close hide the window without destroying it.
	ctrl.SetLightweight(false)
	winBeforeHide := lastWin
	winBeforeHide.CloseWin()
	if winBeforeHide.closed != 0 || winBeforeHide.hidden != 1 || !ctrl.IsHidden() {
		t.Fatalf("expected non-lightweight close to hide window: closed=%d hidden=%d", winBeforeHide.closed, winBeforeHide.hidden)
	}

	// Turning lightweight ON while already hidden immediately destroys the hidden WebView2!
	ctrl.SetLightweight(true)
	if winBeforeHide.closed != 1 {
		t.Fatalf("expected enabling lightweight while hidden to destroy cached window, closed=%d", winBeforeHide.closed)
	}

	// Quitting never cancels window close even when lightweight is false.
	ctrl.Show()
	ctrl.SetLightweight(false)
	ctrl.MarkQuitting()
	lastWin.CloseWin()
	if lastWin.closed != 1 {
		t.Fatalf("expected quitting to allow real close, closed=%d", lastWin.closed)
	}
}

func TestTrayLabelsBilingual(t *testing.T) {
	zh := trayLabelsFor("zh-CN")
	if zh.show != "显示 Ferry" || zh.start != "启动内核" || zh.quit != "退出 Ferry" {
		t.Fatalf("unexpected zh tray labels: %+v", zh)
	}
	en := trayLabelsFor("en-US")
	if en.show != "Show Ferry" || en.start != "Start Core" || en.quit != "Quit Ferry" {
		t.Fatalf("unexpected en tray labels: %+v", en)
	}
}
