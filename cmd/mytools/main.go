// Command mytools is a small developer toolbox built on MyGo's native UI:
// it draws its own window, with no web page.
//
//	go run ./cmd/mytools
package main

import (
	"log"
	"runtime"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"gitee.com/yinshuwei/mytools/internal/app"
)

func main() {
	mygo.App.SetName("DevTools")
	mygo.App.WhenReady(func() {
		// No menu: on Linux and Windows the bar would take a line off the
		// top of the window, and this window is only its pages. On a Mac
		// the same items live in the system's menu bar, where Quit and
		// the rest belong and where they cost the window no room. The
		// keys in the pages work either way, cutting and pasting among
		// them.
		var menu *mygo.Menu
		if runtime.GOOS == "darwin" {
			menu = mygo.NewMenu([]*mygo.MenuItem{
				{Role: mygo.RoleAppMenu},
				{Role: mygo.RoleEditMenu},
				{Role: mygo.RoleViewMenu},
				{Role: mygo.RoleWindowMenu},
			})
		}
		mygo.App.SetMenu(menu) // without this call, mygo installs its own
		a := app.New()
		win := mygo.NewWindow(mygo.WindowOptions{
			Title:    "开发者工具箱",
			Width:    980,
			Height:   700,
			MinWidth: 720, MinHeight: 520,
			StateKey: "main",
			Content:  ui.View(a.View),
		})
		a.SetWindow(win)
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
