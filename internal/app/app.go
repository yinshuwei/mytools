// Package app is the toolbox window: the sidebar of tools, and the page
// shown.
package app

import (
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

	settings bool   // the settings panel is up
	panel    int    // 0 帮助, 1 关于
	help     string // what Help shows: the readme, carried in the binary
}

// New makes the toolbox with every tool.
func New() *App {
	a := &App{tools: tools.All(), help: mytools.Readme}
	a.selected = a.tools[0].ID()
	return a
}

// SetWindow gives the window to the tools that act on it.
func (a *App) SetWindow(win *mygo.Window) {
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

// View builds the window: the sidebar on the left, the page on the right,
// and the settings over both when they are up.
func (a *App) View(c *ui.Context) {
	t := c.Theme()
	// Cmd+, opens the settings, as Preferences does elsewhere.
	if c.Shortcut(ui.Cmd, ui.KeyComma) {
		a.settings = true
	}
	cur := a.current()
	ui.Row(c).Fill().AlignItems(ui.Stretch).Children(func() {
		ui.Column(c).Width(190).Shrink(0).PaddingY(12).Gap(6).Background(t.Surface).Children(func() {
			ui.Row(c).PaddingX(16).Children(func() {
				ui.Text(c, "开发者工具箱").Bold().FontSize(15)
			})
			ui.Sidebar(c, &a.selected, func() {
				for _, tl := range a.tools {
					ui.SidebarItem(c, tl.ID(), nil, tl.Name())
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
				ui.Text(c, cur.Name()).Bold().FontSize(16)
				ui.Text(c, cur.Desc()).FontSize(12).TextColor(t.TextMuted)
			})
			if fh, ok := cur.(tools.FillsHeight); ok && fh.FillsHeight() {
				ui.Column(c).Grow(1).FillWidth().Padding(16, 20).Children(func() {
					cur.View(c)
				})
			} else {
				ui.Scroll(c).Grow(1).FillWidth().Children(func() {
					ui.Column(c).Padding(16, 20, 24, 20).Gap(14).FillWidth().Children(func() {
						cur.View(c)
					})
				})
			}
		})
	})
	a.settingsView(c, t)
}

// settingsView is the settings panel over the pages: what the toolbox says
// about itself. Escape and a click outside close it, as a modal does.
func (a *App) settingsView(c *ui.Context, t *ui.Theme) {
	if !a.settings {
		return
	}
	// As large as the window has room for, so that the readme has a page
	// to scroll in whatever it is read on.
	w, h := c.Size()
	ui.Modal(c, &a.settings, func() {
		ui.Column(c).Gap(10).Width(min(w-100, 760)).Height(min(h-140, 560)).Children(func() {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "设置").FontSize(17).Bold()
				ui.Spacer(c)
				ui.Segmented(c.Key("panel"), &a.panel, "帮助", "关于")
			})
			ui.Divider(c)
			if a.panel == 0 {
				// The readme as it is on disk, in a character of one
				// width, so that its tables keep their shape.
				ui.TextArea(c.Key("help"), &a.help).Label("帮助").
					ReadOnly(true).Font("monospace").FontSize(12).FillWidth().Grow(1)
			} else {
				ui.Scroll(c).Grow(1).FillWidth().Children(func() { a.about(c, t) })
			}
			ui.Row(c).Justify(ui.End).Children(func() {
				if ui.PrimaryButton(c.Key("done"), "完成").Clicked() {
					a.settings = false
				}
			})
		})
	})
}

// about says what the toolbox is, what it was built with, and how it is
// licensed.
func (a *App) about(c *ui.Context, t *ui.Theme) {
	ui.Column(c).Gap(5).FillWidth().MinWidth(0).Children(func() {
		ui.Text(c, "开发者工具箱").FontSize(15).Bold()
		widgets.Hint(c, mygo.App.Name()+" "+mygo.App.Version())
		ui.Text(c, "一个用 mygo 的 Native UI 写的桌面小工具集：窗口由 Go 直接绘制，没有网页前端。").FontSize(13)
		ui.Text(c, "MIT 许可 · Copyright © 2026 yinshuwei").FontSize(13)
		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
			widgets.Hint(c, "源码")
			ui.Link(c, "gitee.com/yinshuwei/mytools", "https://gitee.com/yinshuwei/mytools").FontSize(13)
		})
		ui.Divider(c)
		widgets.Hint(c, "依赖")
		for _, dep := range buildDeps() {
			ui.Text(c, dep).Font("monospace").FontSize(12).TextColor(t.TextMuted).SingleLine()
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
