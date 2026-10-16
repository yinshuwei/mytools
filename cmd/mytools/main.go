// Command mytools is a small developer toolbox built on MyGo's native UI:
// it draws its own window, with no web page.
//
//	go run ./cmd/mytools
package main

import (
	"log"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"gitee.com/yinshuwei/mytools/internal/app"
)

func main() {
	mygo.App.SetName("DevTools")
	mygo.App.WhenReady(func() {
		mygo.App.SetMenu(mygo.NewMenu([]*mygo.MenuItem{
			{Role: mygo.RoleAppMenu},
			{Role: mygo.RoleEditMenu},
			{Role: mygo.RoleViewMenu},
			{Role: mygo.RoleWindowMenu},
		}))
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
