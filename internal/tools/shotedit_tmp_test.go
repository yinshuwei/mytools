package tools

import (
	"image"
	"image/color"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/yinshuwei/mytools/internal/shots"
)

func TestShotDragTMP(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 200, 100))
	for y := 0; y < 100; y++ {
		for x := 0; x < 200; x++ {
			img.SetRGBA(x, y, color.RGBA{R: 20, G: 40, B: 200, A: 255})
		}
	}
	s := &Screenshot{}
	s.SetImage(img, 1)
	tr := ui.NewTester(s.View, 600, 400)
	tr.Frame()
	// Where is the canvas? The picture is fitted into it.
	t.Logf("view=%+v k=%v", s.view, s.k)
	if s.k <= 0 {
		t.Fatal("没有画出图片")
	}
	box, ok := tr.Find("画布")
	if !ok {
		t.Fatal("找不到画布")
	}
	t.Logf("canvas=%+v", box)
	// Drag from picture (20,20) to (120,80), in window coordinates.
	x0 := box.X + s.view.X + 20*s.k
	y0 := box.Y + s.view.Y + 20*s.k
	x1 := box.X + s.view.X + 120*s.k
	y1 := box.Y + s.view.Y + 80*s.k
	tr.Press(x0, y0)
	tr.Move(x1, y1)
	tr.Release(x1, y1)
	tr.Frame()
	t.Logf("shapes=%d", len(s.shapes))
	if len(s.shapes) != 1 {
		t.Fatalf("拖动后 shapes=%d，want 1", len(s.shapes))
	}
	if s.shapes[0].Rect != image.Rect(20, 20, 120, 80) {
		t.Errorf("矩形 = %v", s.shapes[0].Rect)
	}
	out := shots.Rasterize(img, img.Bounds(), s.shapes)
	changed := 0
	for y := 0; y < 100; y++ {
		for x := 0; x < 200; x++ {
			if out.RGBAAt(x, y) != img.RGBAAt(x, y) {
				changed++
			}
		}
	}
	t.Logf("changed=%d", changed)
	if changed == 0 {
		t.Error("导出里没有标注")
	}
}
