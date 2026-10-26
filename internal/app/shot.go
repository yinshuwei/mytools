package app

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"log"
	"math"
	"runtime"

	"github.com/egoist/mygo"

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
	if a.win != nil {
		a.win.Show()
		a.win.Focus()
	}
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

func (a *App) setupTray() {
	menu := mygo.NewMenu([]*mygo.MenuItem{
		{Label: "显示主窗口", Click: func(*mygo.MenuItem, *mygo.Window) { a.showWindow() }},
		{Label: "截图", Click: func(*mygo.MenuItem, *mygo.Window) { a.TakeShot() }},
		mygo.Separator(),
		{Label: "退出", Click: func(*mygo.MenuItem, *mygo.Window) { a.quit() }},
	})
	opts := mygo.TrayOptions{
		Icon:           trayIcon(),
		IconIsTemplate: runtime.GOOS == "darwin",
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
	a.tray, a.trayMenu = tray, menu
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

// trayIcon draws a small camera: the tray has no image file to load, and a
// drawn glyph stays sharp at any scale. macOS tints a template icon itself,
// so there it is black and transparent.
func trayIcon() []byte {
	const size = 32
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	ink := color.NRGBA{R: 79, G: 124, B: 255, A: 255}
	if runtime.GOOS == "darwin" {
		ink = color.NRGBA{A: 255}
	}
	fill := func(r image.Rectangle) {
		draw.Draw(img, r, image.NewUniform(ink), image.Point{}, draw.Src)
	}
	// Camera body, and the bump that holds the viewfinder.
	fill(image.Rect(5, 10, 27, 12))
	fill(image.Rect(5, 23, 27, 25))
	fill(image.Rect(5, 12, 7, 23))
	fill(image.Rect(25, 12, 27, 23))
	fill(image.Rect(12, 6, 20, 10))
	// The lens, a ring two pixels thick.
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			d := math.Hypot(float64(x)+0.5-16, float64(y)+0.5-18)
			if d >= 4.5 && d <= 6.5 {
				img.Set(x, y, ink)
			}
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil
	}
	return buf.Bytes()
}
