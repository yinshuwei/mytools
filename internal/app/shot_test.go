package app

import (
	"image/png"
	"os"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestShotApp(t *testing.T) {
	a := New()
	a.selected = "json"
	tr := ui.NewTester(a.View, 980, 700)
	tr.ClickAt(330, 260)
	tr.Type(`{"user":{"name":"Ada","tags":["dev","math"]},"age":36,"ok":true,"none":null,"list":[{"a":1},{"b":[2,3]}]}`)
	f, err := os.Create("/tmp/app.png")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, tr.Image()); err != nil {
		t.Fatal(err)
	}
}
