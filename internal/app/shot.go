package app

import (
	"image"
	"log"
	"runtime"

	"github.com/egoist/mygo"

	mytools "github.com/yinshuwei/mytools"
	"github.com/yinshuwei/mytools/internal/shots"
)

// TakeShot starts a screenshot, whatever asked for it: the page's button,
// the tray menu, or the global shortcut.
func (a *App) TakeShot() {
	if a.shots != nil {
		a.shots.Start()
	}
}

// editShot shows the picture the overlay chose in the screenshot page, where
// it is annotated, copied or saved.
func (a *App) editShot(img *image.RGBA, scale float64) {
	if a.shotTool == nil {
		return
	}
	a.shotTool.SetImage(img, scale)
	a.selected = a.shotTool.ID()
	a.picked = a.selected
	a.settings = false
	a.showWindow()
}

// SetupDesktop gives the toolbox its desktop manners: the global shortcut,
// the tray icon, and the screenshot service. It runs once, after the window
// exists.
func (a *App) SetupDesktop() {
	if a.win == nil {
		return
	}
	a.shots = shots.New(a.win)
	a.shots.Deliver = a.editShot
	if a.shotTool != nil {
		a.shotTool.Shot = a.TakeShot
		if err := a.shots.Problem(); err != nil {
			a.shotTool.Problem = shots.Friendly(err)
		}
	}
	a.applyShortcuts()
	a.watchClose()
	a.setupTray()
}

// trayMenu is the icon's menu: the window comes back, a screenshot starts,
// and hiding puts the window away — closing the program is not on it.
func (a *App) trayMenu() *mygo.Menu {
	return mygo.NewMenu([]*mygo.MenuItem{
		{Label: "显示主窗口", Click: func(*mygo.MenuItem, *mygo.Window) { a.showWindow() }},
		{Label: "截图", Click: func(*mygo.MenuItem, *mygo.Window) { a.TakeShot() }},
		mygo.Separator(),
		{Label: "隐藏主窗口", Click: func(*mygo.MenuItem, *mygo.Window) { a.HideWindow() }},
	})
}

func (a *App) setupTray() {
	menu := a.trayMenu()
	opts := mygo.TrayOptions{
		Icon:           mytools.Icon,
		IconIsTemplate: false,
		ToolTip:        "开发者工具箱",
	}
	// The icon is the window's way back: a click on it opens the window, and
	// the menu is what a right click is for — as far as the platform allows
	// it, which trayMenuAttached and trayClickOpensWindow tell.
	if trayMenuAttached(runtime.GOOS) {
		opts.Menu = menu
	}
	tray, err := mygo.NewTray(opts)
	if err != nil {
		// A tray is a convenience, and some Linux desktops have no
		// AppIndicator host. The shortcut and the page still work.
		log.Printf("托盘图标不可用：%v", err)
		return
	}
	a.tray, a.macMenu = tray, menu
	if trayClickOpensWindow(runtime.GOOS) {
		tray.OnClick(a.showWindow)
	}
	if runtime.GOOS == "darwin" {
		tray.OnRightClick(a.popTrayMenu)
	}
}

// trayMenuAttached says whether the icon carries its menu itself. An
// AppIndicator on Linux has nothing but a menu — a click on it is a click on
// the menu — and Windows shows an attached menu on a right click, which
// leaves the left click to the app. A Mac's status item with a menu would
// show that menu on the click itself, so there the menu is kept aside and
// popped on a right click.
func trayMenuAttached(goos string) bool { return goos != "darwin" }

// trayClickOpensWindow says whether a click on the icon reaches the app, which
// is what opens the window. AppIndicator reports no clicks at all.
func trayClickOpensWindow(goos string) bool { return goos == "windows" || goos == "darwin" }

// defaultShotKey is the screenshot shortcut a fresh settings file starts with,
// in the form the recorder writes, so that a key the user picks compares equal
// to the one it replaces.
func defaultShotKey() string {
	if runtime.GOOS == "darwin" {
		return "Super+Shift+A"
	}
	return "Ctrl+Alt+A"
}
