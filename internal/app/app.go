// Package app is the toolbox window: the sidebar of tools, and the page
// shown.
package app

import (
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"gitee.com/yinshuwei/mytools/internal/tools"
)

// App is the toolbox.
type App struct {
	tools    []tools.Tool
	selected string // the ID of the tool shown
}

// New makes the toolbox with every tool.
func New() *App {
	a := &App{tools: tools.All()}
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

// View builds the window: the sidebar on the left, the page on the right.
func (a *App) View(c *ui.Context) {
	t := c.Theme()
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
		})
		ui.Column(c).Grow(1).MinWidth(0).FillHeight().Children(func() {
			ui.Column(c).Padding(14, 20).Gap(2).FillWidth().BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
				ui.Text(c, cur.Name()).Bold().FontSize(16)
				ui.Text(c, cur.Desc()).FontSize(12).TextColor(t.TextMuted)
			})
			ui.Scroll(c).Grow(1).FillWidth().Children(func() {
				ui.Column(c).Padding(16, 20, 24, 20).Gap(14).FillWidth().Children(func() {
					cur.View(c)
				})
			})
		})
	})
}
