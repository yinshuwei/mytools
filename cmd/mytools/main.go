// Command mytools is a small developer toolbox built on MyGo's native UI:
// it draws its own window, with no web page.
//
//	go run ./cmd/mytools
package main

import (
	"log"
	"os"
	"runtime"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"gitee.com/yinshuwei/mytools/internal/app"
)

// version is what the About panel shows; a build may set it with
//
//	go build -ldflags "-X main.version=1.2.3"
var version = "0.1.0"

func main() {
	// WebKit draws through the GPU unless told not to. On a machine where
	// the window itself draws without one — a VM, a container, a remote
	// display — starting the web engine that way leaves the GL context
	// the window presents with broken, and the process aborts outright
	// (epoxy: "Couldn't find current GLX or EGL context"). The help page
	// is text; it loses nothing drawn on the CPU.
	if os.Getenv("WEBKIT_DISABLE_COMPOSITING_MODE") == "" {
		_ = os.Setenv("WEBKIT_DISABLE_COMPOSITING_MODE", "1")
	}
	mygo.App.SetName("MyTools")
	mygo.App.SetVersion(version)
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
