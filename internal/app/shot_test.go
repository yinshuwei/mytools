package app

import (
	"image/png"
	"os"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestShotApp(t *testing.T) {
	a := New()
	a.selected = "clock"
	tr := ui.NewTester(a.View, 980, 700)
	if err := tr.Click("本地"); err != nil {
		t.Fatal(err)
	}
	if err := tr.Click("Asia/Tokyo"); err != nil {
		t.Fatal(err)
	}
	if err := tr.Click("填入当前"); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create("/tmp/app.png")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, tr.Image()); err != nil {
		t.Fatal(err)
	}
}
