package app

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// TestShots writes the screenshots the readme shows, drawn by the real
// pages in a window that is never shown:
//
//	MYTOOLS_SHOTS=1 go test ./internal/app -run TestShots
//
// They are written to docs/, beside the readme that shows them, and are to
// be looked at before being kept: a page that changed may be one worth
// shooting again. Without MYTOOLS_SHOTS it does nothing, so that plain
// `go test ./...` does not rewrite files of the repository.
func TestShots(t *testing.T) {
	if os.Getenv("MYTOOLS_SHOTS") == "" {
		t.Skip("设 MYTOOLS_SHOTS=1 才写截图")
	}
	const dir = "../../docs"
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	// The name and version the About panel shows, as main sets them.
	mygo.App.SetName("MyTools")
	mygo.App.SetVersion("0.1.0")
	shot := func(name string, w, h int, drive func(a *App, tr *ui.Tester)) {
		t.Helper()
		a := New()
		tr := ui.NewTester(a.View, w, h)
		tr.SetDark(true)
		tr.Frame()
		drive(a, tr)
		tr.Frame()
		f, err := os.Create(filepath.Join(dir, name+".png"))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tr.Image()); err != nil {
			t.Fatal(err)
		}
		t.Logf("wrote %s.png", name)
	}
	// Clicking a tool in the sidebar and typing where it says.
	click := func(tr *ui.Tester, what string) {
		t.Helper()
		if err := tr.Click(what); err != nil {
			t.Fatal(err)
		}
	}

	shot("json", 1000, 680, func(a *App, tr *ui.Tester) {
		click(tr, "JSON 输入")
		tr.Type(`{"name":"Ada","age":36,"admin":true,"tags":["dev","go"],"address":{"city":"Shanghai","zip":"200000"},"score":1.3}`)
		tr.Key(ui.Cmd, ui.KeyF)
		tr.Type("name")
	})

	shot("calc", 1000, 680, func(a *App, tr *ui.Tester) {
		click(tr, "计算器")
		tr.Type("1.3*12-12")
		tr.Key(0, ui.KeyEnter)
		tr.Type("sqrt(2)*10")
		tr.Key(0, ui.KeyEnter)
		tr.Type("2^16")
		tr.Key(0, ui.KeyEnter)
		tr.Type("(1+2)*3")
	})

	shot("qr", 1000, 680, func(a *App, tr *ui.Tester) {
		click(tr, "二维码")
		click(tr, "二维码内容")
		tr.Type("https://gitee.com/yinshuwei/mytools")
	})

	shot("settings", 1000, 680, func(a *App, tr *ui.Tester) {
		a.settings, a.panel = true, 2
	})
}
