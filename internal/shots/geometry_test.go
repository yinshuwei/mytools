package shots

import (
	"image"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestRectFromDrag(t *testing.T) {
	// A drag is a rectangle between its corners, however it was dragged.
	cases := []struct {
		name           string
		x0, y0, x1, y1 float32
		want           ui.Rect
	}{
		{"right and down", 10, 20, 30, 50, ui.Rect{X: 10, Y: 20, W: 20, H: 30}},
		{"left and up", 30, 50, 10, 20, ui.Rect{X: 10, Y: 20, W: 20, H: 30}},
		{"left and down", 30, 20, 10, 50, ui.Rect{X: 10, Y: 20, W: 20, H: 30}},
		{"right and up", 10, 50, 30, 20, ui.Rect{X: 10, Y: 20, W: 20, H: 30}},
		{"a click", 7, 7, 7, 7, ui.Rect{X: 7, Y: 7}},
	}
	for _, c := range cases {
		if got := rectFromDrag(c.x0, c.y0, c.x1, c.y1); got != c.want {
			t.Errorf("%s: got %+v, want %+v", c.name, got, c.want)
		}
	}
}

func TestClampSelection(t *testing.T) {
	cases := []struct {
		name string
		sel  ui.Rect
		w, h float32
		want ui.Rect
	}{
		{"inside", ui.Rect{X: 10, Y: 10, W: 50, H: 40}, 100, 80, ui.Rect{X: 10, Y: 10, W: 50, H: 40}},
		{"right edge", ui.Rect{X: 80, Y: 10, W: 50, H: 20}, 100, 80, ui.Rect{X: 80, Y: 10, W: 20, H: 20}},
		{"bottom edge", ui.Rect{X: 10, Y: 70, W: 20, H: 50}, 100, 80, ui.Rect{X: 10, Y: 70, W: 20, H: 10}},
		{"past both", ui.Rect{X: -10, Y: -10, W: 200, H: 200}, 100, 80, ui.Rect{X: 0, Y: 0, W: 100, H: 80}},
		{"whole area", ui.Rect{X: 0, Y: 0, W: 100, H: 80}, 100, 80, ui.Rect{X: 0, Y: 0, W: 100, H: 80}},
	}
	for _, c := range cases {
		if got := clampSelection(c.sel, c.w, c.h); got != c.want {
			t.Errorf("%s: got %+v, want %+v", c.name, got, c.want)
		}
	}
}

func TestCropRect(t *testing.T) {
	// A 1000×500 content area showing a 2000×1000 picture: two pixels a
	// DIP, as on a HiDPI screen.
	content := ui.Rect{W: 1000, H: 500}
	img := image.Rect(0, 0, 2000, 1000)

	if got := cropRect(ui.Rect{X: 100, Y: 50, W: 200, H: 100}, content, img); got != image.Rect(200, 100, 600, 300) {
		t.Errorf("2x: got %v, want (200,100)-(600,300)", got)
	}
	// Rounding keeps a selection on pixel bounds, and the crop is clipped
	// to the picture.
	if got := cropRect(ui.Rect{X: 0, Y: 0, W: 1000, H: 500}, content, img); got != img {
		t.Errorf("whole: got %v, want %v", got, img)
	}
	if got := cropRect(ui.Rect{X: 999.6, Y: 499.6, W: 10, H: 10}, content, img); got != image.Rect(1999, 999, 2000, 1000) {
		t.Errorf("past edge: got %v, want (1999,999)-(2000,1000)", got)
	}
	if got := cropRect(ui.Rect{}, content, img); !got.Empty() {
		t.Errorf("empty selection: got %v, want empty", got)
	}
	if got := cropRect(ui.Rect{X: 10, Y: 10, W: 10, H: 10}, ui.Rect{}, img); !got.Empty() {
		t.Errorf("empty content: got %v, want empty", got)
	}
}

func TestPixelSize(t *testing.T) {
	// The label says the size in the display's pixels, not in DIPs.
	w, h := pixelSize(
		ui.Rect{X: 10, Y: 10, W: 100, H: 50},
		ui.Rect{W: 500, H: 250},
		image.Rect(0, 0, 1500, 750),
	)
	if w != 300 || h != 150 {
		t.Errorf("got %d×%d, want 300×150", w, h)
	}
}

func TestSaveName(t *testing.T) {
	at := time.Date(2026, 10, 23, 9, 8, 7, 0, time.Local)
	if got, want := saveName(at), "screenshot-20261023-090807.png"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
