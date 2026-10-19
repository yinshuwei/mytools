// Package app is the toolbox window: the sidebar of tools, and the page
// shown.
package app

import (
	"errors"
	"fmt"
	"math"
	"runtime/debug"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	mytools "gitee.com/yinshuwei/mytools"
	"gitee.com/yinshuwei/mytools/internal/tools"
	"gitee.com/yinshuwei/mytools/internal/widgets"
)

// App is the toolbox.
type App struct {
	tools    []tools.Tool
	selected string // the ID of the tool shown

	win      *mygo.Window
	picked   string // the tool the sidebar had last, to tell a pick from a redraw
	settings bool   // the settings page is the one shown
	panel    int    // 0 外观, 1 帮助, 2 关于
	help     string // the readme, carried in the binary

	// themeMode is "light", "dark" or "system", and zoom how much larger
	// than usual the pages are drawn: the appearance the user chose.
	themeMode string
	zoom      float64

	// helpWeb is the page the help panel shows the readme in, made the
	// first time it is asked for rather than at startup: a web view is a
	// whole engine, and most of a session never opens the settings.
	helpWeb *mygo.WebView
	helpKey string // the colors it was last loaded with
	helpErr string // why the readme is shown as text instead

	// helpMaking is set while the engine is coming up beside the frames,
	// and helpMade carries what it came up with.
	helpMaking bool
	helpMade   chan helpStart
}

// helpStart is what making the help page came to.
type helpStart struct {
	view *mygo.WebView
	err  error
}

// New makes the toolbox with every tool.
func New() *App {
	a := &App{
		tools:     tools.All(),
		help:      mytools.Readme,
		helpMade:  make(chan helpStart, 1),
		themeMode: "system",
		zoom:      1,
	}
	a.selected = a.tools[0].ID()
	a.picked = a.selected
	return a
}

// SetWindow gives the window to the app itself and to the tools that act on
// it, and takes the appearance the last run left.
func (a *App) SetWindow(win *mygo.Window) {
	a.win = win
	a.loadSettings()
	for _, t := range a.tools {
		if w, ok := t.(tools.WindowAware); ok {
			w.SetWindow(win)
		}
	}
}

// current returns the tool being shown.
func (a *App) current() tools.Tool {
	for _, t := range a.tools {
		if t.ID() == a.selected {
			return t
		}
	}
	return a.tools[0]
}

// applyAppearance dresses the frame the way the user chose: the colors, and
// the zoom over them. It runs before anything is built, as SetTheme only
// reaches the widgets made after it.
func (a *App) applyAppearance(c *ui.Context) {
	z := float32(a.zoom)
	if z <= 0 {
		z = 1
	}
	theme := *c.Theme()
	switch a.themeMode {
	case "light":
		theme = *ui.LightTheme()
	case "dark":
		theme = *ui.DarkTheme()
	}
	if a.themeMode != "system" {
		// An appearance of its own still follows the text size the
		// desktop asks for.
		if p := c.Preferences(); p.TextScale > 0 && p.TextScale != 1 {
			theme.FontSize *= p.TextScale
		}
	}
	theme.FontSize *= z
	theme.Spacing *= z
	theme.Radius *= z
	theme.ScrollbarWidth = 6 * z
	c.SetTheme(&theme)
	widgets.SetZoom(z)
}

// View builds the window: the sidebar on the left, the page on the right.
// The settings are a page of their own rather than a panel over one: what
// they show — a readme — wants the room, and a page can be left by picking
// a tool in the sidebar, as a panel cannot.
func (a *App) View(c *ui.Context) {
	a.applyAppearance(c)
	t := c.Theme()
	// What the help page's engine came up with, made while the frames
	// went on.
	widgets.Drain(a.helpMade, func(made helpStart) {
		a.helpMaking = false
		if made.err != nil {
			a.helpErr = made.err.Error()
			return
		}
		a.helpWeb, a.helpKey = made.view, ""
	})
	// Cmd+, opens the settings, as Preferences does elsewhere.
	if c.Shortcut(ui.Cmd, ui.KeyComma) {
		a.settings = true
	}
	// Moving the selection with the keyboard is leaving the settings too:
	// the page the sidebar names is what the window is for.
	if a.selected != a.picked {
		a.picked, a.settings = a.selected, false
	}
	// Escape leaves them too; nothing else is up to take it, the pages
	// being unbuilt while the settings show.
	if a.settings && c.Shortcut(0, ui.KeyEscape) {
		a.settings = false
	}
	ui.Row(c).Fill().AlignItems(ui.Stretch).Children(func() {
		ui.Column(c).Width(widgets.Scaled(190)).Shrink(0).PaddingY(12).Gap(6).Background(t.Surface).Children(func() {
			ui.Row(c).PaddingX(16).Children(func() {
				ui.Text(c, "开发者工具箱").Bold().FontSize(widgets.Scaled(15))
			})
			ui.Sidebar(c, &a.selected, func() {
				for _, tl := range a.tools {
					// Any click is a way out of the settings, the item
					// already picked included: a sidebar picks pages,
					// and setting the same one again still shows it.
					if ui.SidebarItem(c, tl.ID(), nil, tl.Name()).Clicked() {
						a.settings = false
					}
				}
			}).Grow(1).Label("工具")
			// In the corner under the tools, out of the way of them.
			ui.Row(c).PaddingX(10).Children(func() {
				if ui.Button(c.Key("settings"), "设置").FillWidth().
					Tooltip("帮助与关于（" + widgets.ShortcutHint(0, ",") + "）").Clicked() {
					a.settings = true
				}
			})
		})
		ui.Column(c).Grow(1).MinWidth(0).FillHeight().Children(func() {
			ui.Column(c).Padding(14, 20).Gap(2).FillWidth().BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
				name, desc := a.current().Name(), a.current().Desc()
				if a.settings {
					name, desc = "设置", "外观、帮助与关于"
				}
				ui.Text(c, name).Bold().FontSize(widgets.Scaled(16))
				ui.Text(c, desc).FontSize(widgets.Scaled(12)).TextColor(t.TextMuted)
			})
			a.page(c, t)
		})
	})
}

// page fills the room under the heading: the settings, or the tool the
// sidebar has picked.
func (a *App) page(c *ui.Context, t *ui.Theme) {
	if a.settings {
		ui.Column(c).Grow(1).FillWidth().Padding(16, 20).Children(func() {
			a.settingsPage(c, t)
		})
		return
	}
	cur := a.current()
	// A page of controls asks for a width of its own: stretched across a
	// very wide window, the buttons at its right are a journey from what
	// they act on. It stays at the left, under the heading.
	maxWidth := float32(0)
	if mw, ok := cur.(tools.MaxWidth); ok {
		maxWidth = mw.MaxWidth()
	}
	if fh, ok := cur.(tools.FillsHeight); ok && fh.FillsHeight() {
		page := ui.Column(c).Grow(1).FillWidth().Padding(16, 20)
		if maxWidth > 0 {
			page.MaxWidth(maxWidth)
		}
		page.Children(func() { cur.View(c) })
		return
	}
	ui.Scroll(c).Grow(1).FillWidth().Children(func() {
		page := ui.Column(c).Padding(16, 20, 24, 20).Gap(14).FillWidth()
		if maxWidth > 0 {
			page.MaxWidth(maxWidth)
		}
		page.Children(func() { cur.View(c) })
	})
}

// settingsPage is what the toolbox says about itself: how it looks, the
// readme, or what it is and what it is built with.
func (a *App) settingsPage(c *ui.Context, t *ui.Theme) {
	ui.Column(c).FillWidth().FillHeight().Gap(10).Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Segmented(c.Key("panel"), &a.panel, "外观", "帮助", "关于")
			ui.Spacer(c)
			if ui.Button(c.Key("back"), "回到工具").Tooltip("也可以按 Esc，或点左边的工具").
				Clicked() {
				a.settings = false
			}
		})
		switch a.panel {
		case 0:
			a.appearancePage(c, t)
		case 1:
			a.helpView(c, t)
		default:
			ui.Scroll(c).Grow(1).FillWidth().Children(func() { a.about(c, t) })
		}
	})
}

// appearance is the settings of how the window looks: the colors, and how
// large the pages are drawn.
func (a *App) appearancePage(c *ui.Context, t *ui.Theme) {
	modes := []string{"浅色", "深色", "跟随系统"}
	chosen := 2
	switch a.themeMode {
	case "light":
		chosen = 0
	case "dark":
		chosen = 1
	}
	ui.Column(c).Gap(14).FillWidth().Children(func() {
		widgets.Field(c, "颜色", func() {
			// In a row of its own, so that the control is as wide as its
			// segments rather than as wide as the page.
			ui.Row(c).Children(func() {
				seg := ui.Segmented(c.Key("mode"), &chosen, modes...)
				if seg.Changed() {
					a.themeMode = []string{"light", "dark", "system"}[chosen]
					a.saveSettings()
				}
			})
		})
		widgets.Field(c, "缩放比例", func() {
			ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
				slider := ui.Slider(c.Key("zoom"), &a.zoom, minZoom, maxZoom).Grow(1).
					Label("缩放比例").Tooltip("拖动缩放整个页面，文字、图标与控件一起变")
				if slider.Changed() {
					a.zoom = snapZoom(a.zoom)
					a.saveSettings()
				}
				ui.Text(c, fmt.Sprintf("%d%%", int(math.Round(a.zoom*100)))).
					Bold().FontSize(widgets.Scaled(13)).MinWidth(widgets.Scaled(46))
				reset := ui.Button(c.Key("zoomreset"), "还原 100%").Disabled(a.zoom == 1).
					Tooltip("回到桌面的常规大小")
				if reset.Clicked() {
					a.zoom = 1
					a.saveSettings()
				}
			})
		})
		widgets.Hint(c, "缩放同时放大文字、图标与控件，立即生效，下次启动仍然记得。")
		if a.themeMode == "system" {
			widgets.Hint(c, "「跟随系统」用桌面现在的浅色 / 深色，主题色也随之跟着桌面。")
		}
	})
}

// helpView shows the readme: rendered from markdown into a web view, or, on
// a machine where no web view can be had, as the text it is.
func (a *App) helpView(c *ui.Context, t *ui.Theme) {
	view, err := a.helpPageView(t)
	switch {
	case view != nil:
		ui.WebView(c, view).Label("帮助").Grow(1).Radius(8).Border(1, t.Border)
	case errors.Is(err, errHelpStarting):
		ui.Column(c).Grow(1).Center().Children(func() {
			widgets.Hint(c, "正在准备帮助页…")
		})
	default:
		if a.helpErr == "" {
			a.helpErr = err.Error()
		}
		widgets.Hint(c, "这里没有可用的 web view（"+a.helpErr+"），下面是原文：")
		// Read-only, and in a character of one width, so that the
		// tables of the readme keep their shape.
		ui.TextArea(c.Key("help"), &a.help).Label("帮助").
			ReadOnly(true).Font("monospace").FontSize(widgets.Scaled(12)).FillWidth().Grow(1)
	}
}

// errHelpStarting says the page is being made, which is no failure: the
// panel says so and asks again on the frame after.
var errHelpStarting = errors.New("正在准备")

// helpPageView returns the page the readme is shown in, starting it when it
// is not up yet.
//
// The engine is brought up beside the frames and not inside one: it takes a
// moment, and — with GL — a web engine made in the middle of a frame leaves
// the context the frame is presented with in a state that aborts the whole
// process. What it makes is taken on the frame after, through helpMade.
func (a *App) helpPageView(t *ui.Theme) (*mygo.WebView, error) {
	if a.helpWeb != nil && !a.helpWeb.IsDestroyed() {
		// Dress it again when the window's colors change under it.
		if style := helpStyle(t); style != a.helpKey {
			a.helpKey = style
			a.helpWeb.Page().LoadHTML(helpDocument(style), helpBase())
		}
		return a.helpWeb, nil
	}
	if a.helpErr != "" {
		return nil, errors.New(a.helpErr)
	}
	if helpAsText {
		return nil, errors.New("MYTOOLS_NO_WEBVIEW 开着")
	}
	if a.helpMaking {
		return nil, errHelpStarting
	}
	if a.win == nil {
		return nil, errors.New("窗口还没有建好")
	}
	a.helpMaking = true
	win, made := a.win, a.helpMade
	bg := cssColor(t.Background)
	go func() {
		view, err := win.NewWebView(mygo.WebViewOptions{BackgroundColor: bg})
		made <- helpStart{view: view, err: err}
		win.Invalidate()
	}()
	return nil, errHelpStarting
}

// about says what the toolbox is, what it was built with, and how it is
// licensed.
func (a *App) about(c *ui.Context, t *ui.Theme) {
	ui.Column(c).Gap(5).FillWidth().MinWidth(0).Children(func() {
		ui.Text(c, "开发者工具箱").FontSize(widgets.Scaled(15)).Bold()
		widgets.Hint(c, mygo.App.Name()+" "+mygo.App.Version())
		ui.Text(c, "一个用 mygo 的 Native UI 写的桌面小工具集：窗口由 Go 直接绘制，没有网页前端。").FontSize(widgets.Scaled(13))
		ui.Text(c, "MIT 许可 · Copyright © 2026 yinshuwei").FontSize(widgets.Scaled(13))
		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
			widgets.Hint(c, "源码")
			ui.Link(c, "gitee.com/yinshuwei/mytools", "https://gitee.com/yinshuwei/mytools").FontSize(widgets.Scaled(13))
		})
		ui.Divider(c)
		widgets.Hint(c, "依赖")
		for _, dep := range buildDeps() {
			ui.Text(c, dep).Font("monospace").FontSize(widgets.Scaled(12)).TextColor(t.TextMuted).SingleLine()
		}
	})
}

// buildDeps are the modules this binary was built with, as the build itself
// recorded them: the truth about the running program rather than a list
// written out by hand.
func buildDeps() []string {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return nil
	}
	out := make([]string, 0, len(bi.Deps))
	for _, d := range bi.Deps {
		out = append(out, d.Path+" "+d.Version)
	}
	return out
}
