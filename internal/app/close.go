package app

import (
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

// showWindow calls the toolbox back from the tray.
func (a *App) showWindow() {
	if a.win == nil || a.win.IsDestroyed() {
		return
	}
	a.win.Show()
	a.win.Focus()
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
