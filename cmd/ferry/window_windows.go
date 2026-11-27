//go:build windows

package main

import (
	"sync"
	"sync/atomic"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

type managedWindow interface {
	ShowFocus()
	HideWin()
	CloseWin()
}

type wailsWindowAdapter struct {
	win *application.WebviewWindow
}

func (a *wailsWindowAdapter) ShowFocus() { a.win.Show().Focus() }
func (a *wailsWindowAdapter) HideWin()   { a.win.Hide() }
func (a *wailsWindowAdapter) CloseWin()  { a.win.Close() }

type windowFactory func(onClosing func() bool) managedWindow

// windowController manages WebView2 creation, hiding, and lightweight-mode
// destruction/recreation when minimized to the system tray.
type windowController struct {
	mu          sync.Mutex
	factory     windowFactory
	current     managedWindow
	hidden      atomic.Bool
	lightweight atomic.Bool
	quitting    atomic.Bool
}

func newWindowController(startMinimized, startLightweight bool, factory windowFactory) *windowController {
	c := &windowController{factory: factory}
	c.hidden.Store(startMinimized)
	c.lightweight.Store(startLightweight)
	if !startMinimized || !startLightweight {
		c.current = c.createLocked()
	}
	return c
}

func newDesktopWindowController(desktop *application.App, startMinimized, startLightweight bool) *windowController {
	firstCreation := true
	return newWindowController(startMinimized, startLightweight, func(onClosing func() bool) managedWindow {
		hideInitial := firstCreation && startMinimized
		firstCreation = false
		win := desktop.Window.NewWithOptions(application.WebviewWindowOptions{
			Title: "Ferry", Width: 1180, Height: 780, MinWidth: 900, MinHeight: 620,
			Frameless: true, Hidden: hideInitial,
			BackgroundColour: application.NewRGB(15, 18, 23), URL: "/",
		})
		win.RegisterHook(events.Common.WindowClosing, func(event *application.WindowEvent) {
			if onClosing() {
				event.Cancel()
			}
		})
		return &wailsWindowAdapter{win: win}
	})
}

func (c *windowController) createLocked() managedWindow {
	var created managedWindow
	created = c.factory(func() bool {
		return c.handleClosing(created)
	})
	return created
}

// handleClosing returns true when the close event must be cancelled (hide-only mode).
// When lightweight is true or quitting, it returns false so WebView2 is destroyed.
func (c *windowController) handleClosing(target managedWindow) bool {
	if c.quitting.Load() {
		return false
	}
	c.hidden.Store(true)
	if c.lightweight.Load() {
		c.mu.Lock()
		if c.current == target {
			c.current = nil
		}
		c.mu.Unlock()
		return false
	}
	target.HideWin()
	return true
}

// Show displays the existing WebView2 window or recreates one if destroyed in lightweight mode.
func (c *windowController) Show() (recreated bool) {
	c.mu.Lock()
	win := c.current
	if win == nil {
		win = c.createLocked()
		c.current = win
		recreated = true
	}
	c.hidden.Store(false)
	c.mu.Unlock()
	win.ShowFocus()
	return recreated
}

func (c *windowController) SetLightweight(on bool) {
	c.lightweight.Store(on)
	if !on || !c.hidden.Load() {
		return
	}
	c.mu.Lock()
	win := c.current
	c.current = nil
	c.mu.Unlock()
	if win != nil {
		win.CloseWin()
	}
}

func (c *windowController) MarkQuitting()       { c.quitting.Store(true) }
func (c *windowController) IsHidden() bool      { return c.hidden.Load() }
func (c *windowController) IsLightweight() bool { return c.lightweight.Load() }
