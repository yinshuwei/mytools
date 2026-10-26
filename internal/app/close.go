package app

import (
	"runtime"
	"time"

	"github.com/egoist/mygo"
)

// watchClose puts the window away when it is closed, rather than taking the
// program down with it: the window is not the program, and the icon is the way
// back to it. A machine with no icon to come back from quits instead, as the
// window would be gone for good.
func (a *App) watchClose() {
	if a.win == nil {
		return
	}
	a.win.OnClose(func(e *mygo.CloseEvent) {
		if a.quitting {
			// The tray menu said to leave: this close is the way out.
			return
		}
		e.PreventDefault()
		a.hideToTray()
	})
}

// hideToTray puts the window away and leaves the program running with its
// icon; the screenshot shortcut keeps working while it is away.
func (a *App) hideToTray() {
	if a.tray == nil || a.win == nil || a.win.IsDestroyed() {
		a.quit()
		return
	}
	a.win.Hide()
}

// quit leaves the program. The flag lets the window close, which would
// otherwise only put it away.
func (a *App) quit() {
	a.quitting = true
	if a.quitApp != nil {
		a.quitApp()
		return
	}
	mygo.App.Quit()
}

// toggleWindow is what the shortcut does: it puts the window away when it is
// on screen, and calls it back when it is not.
func (a *App) toggleWindow() {
	if a.win == nil || a.win.IsDestroyed() {
		return
	}
	if windowActionFor(a.win.IsVisible(), a.win.IsMinimized()) == actHide {
		a.hideWindow()
		return
	}
	a.showWindow()
}

// windowAction is what the shortcut does to the window.
type windowAction int

const (
	actShow windowAction = iota
	actHide
)

// windowActionFor is the window's answer: one on screen goes away, one that is
// hidden, or minimized out of the way, comes back.
func windowActionFor(visible, minimized bool) windowAction {
	if visible && !minimized {
		return actHide
	}
	return actShow
}

// hideWindow puts the window away and leaves the program in the tray. A
// machine with no tray keeps its window, minimized, where the taskbar can
// bring it back: one that is hidden there could not be found again.
func (a *App) hideWindow() {
	if a.win == nil || a.win.IsDestroyed() {
		return
	}
	if a.tray == nil {
		a.win.Minimize()
		return
	}
	a.win.Hide()
}

// showWindow calls the toolbox back from the tray, in front of what is on
// screen. Presenting a window from a shortcut is an activation request without
// a time on X11, which the window manager is free to refuse: the window is
// mapped, and only its button in the taskbar shows for it.
func (a *App) showWindow() {
	if a.win == nil || a.win.IsDestroyed() {
		return
	}
	a.win.Show()
	a.win.Focus()
	if runtime.GOOS != "linux" {
		return
	}
	// Raising the window over everything for a moment makes the window
	// manager restack it, which a refused activation does not, and the
	// second focus comes once it is at the top. The flag goes back as it
	// was.
	a.win.SetAlwaysOnTop(true)
	time.AfterFunc(150*time.Millisecond, func() {
		if a.win == nil || a.win.IsDestroyed() {
			return
		}
		a.win.Update(func() {
			a.win.SetAlwaysOnTop(false)
			a.win.Focus()
		})
	})
}

// popTrayMenu shows the menu under the icon on a Mac, where the icon itself
// holds no menu — one that did would show it on the click, and the click
// could never open the window. The menu goes on for as long as it is open and
// comes off again, so that the next click is a click on the window.
func (a *App) popTrayMenu() {
	if a.tray == nil || a.trayMenu == nil {
		return
	}
	a.tray.SetMenu(a.trayMenu)
	a.tray.PopUpMenu()
	a.tray.SetMenu(nil)
}
