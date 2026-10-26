package shots

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"testing"

	"github.com/egoist/mygo/ui"
)

// bluePicture is a blue picture to draw annotations on.
func bluePicture(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(img, img.Bounds(), image.NewUniform(color.RGBA{R: 0, G: 0, B: 255, A: 255}), image.Point{}, draw.Src)
	return img
}

// TestShapeHit checks what a pointer takes: a rectangle by its inside, an
// arrow by its stroke rather than the box around it, a pen by its path, and
// text by where it runs.
func TestShapeHit(t *testing.T) {
	rect := Shape{Tool: ToolRect, Width: 2, Rect: image.Rect(10, 10, 40, 30)}
	if !rect.Hit(image.Pt(25, 20), 0) {
		t.Error("矩形里面没点中")
	}
	if rect.Hit(image.Pt(60, 20), 0) {
		t.Error("矩形外面点中了")
	}

	arrow := Shape{Tool: ToolArrow, Width: 2, From: image.Pt(0, 0), To: image.Pt(100, 100)}
	if !arrow.Hit(image.Pt(50, 50), 1) {
		t.Error("箭头上没点中")
	}
	if arrow.Hit(image.Pt(10, 90), 1) {
		t.Error("箭头的框里、线外也点中了")
	}

	ring := Shape{Tool: ToolEllipse, Width: 2, Rect: image.Rect(0, 0, 100, 60)}
	if !ring.Hit(image.Pt(50, 30), 0) {
		t.Error("圆圈里面没点中")
	}
	if ring.Hit(image.Pt(5, 5), 0) {
		t.Error("圆圈外的角落也点中了")
	}

	pen := Shape{Tool: ToolPen, Width: 4, Points: []image.Point{{0, 0}, {50, 0}, {50, 50}}}
	if !pen.Hit(image.Pt(25, 2), 1) {
		t.Error("画笔线上没点中")
	}
	if pen.Hit(image.Pt(25, 30), 1) {
		t.Error("画笔折线外点中了")
	}

	text := Shape{Tool: ToolText, At: image.Pt(10, 10), Text: "ab", Size: 20}
	if !text.Hit(image.Pt(20, 20), 2) {
		t.Error("文字上没点中")
	}
	if text.Hit(image.Pt(80, 20), 2) {
		t.Error("文字外点中了")
	}
}

// TestShapeMoved checks that every kind of shape moves with its own
// geometry, and that a pen's path is its own.
func TestShapeMoved(t *testing.T) {
	rect := Shape{Tool: ToolRect, Rect: image.Rect(0, 0, 10, 10)}.Moved(5, 7)
	if rect.Rect != image.Rect(5, 7, 15, 17) {
		t.Errorf("矩形移动后 = %v", rect.Rect)
	}
	arrow := Shape{Tool: ToolArrow, From: image.Pt(0, 0), To: image.Pt(5, 5)}.Moved(1, 2)
	if arrow.From != image.Pt(1, 2) || arrow.To != image.Pt(6, 7) {
		t.Errorf("箭头移动后 = %v → %v", arrow.From, arrow.To)
	}
	ring := Shape{Tool: ToolEllipse, Rect: image.Rect(0, 0, 10, 6)}.Moved(2, 3)
	if ring.Rect != image.Rect(2, 3, 12, 9) {
		t.Errorf("圆圈移动后 = %v", ring.Rect)
	}
	text := Shape{Tool: ToolText, At: image.Pt(3, 4)}.Moved(1, 1)
	if text.At != image.Pt(4, 5) {
		t.Errorf("文字移动后 = %v", text.At)
	}
	pen := Shape{Tool: ToolPen, Points: []image.Point{{0, 0}, {4, 4}}}
	moved := pen.Moved(10, 0)
	if moved.Points[0] != image.Pt(10, 0) {
		t.Errorf("画笔移动后 = %v", moved.Points)
	}
	if pen.Points[0] != image.Pt(0, 0) {
		t.Error("移动改了原来的画笔路径")
	}
}

// TestRasterizeRect checks that a rectangle is drawn over the picture and
// that the pixels outside it are the picture's own.
func TestRasterizeRect(t *testing.T) {
	src := bluePicture(60, 40)
	out := Rasterize(src, src.Bounds(), []Shape{{
		Tool: ToolRect, Color: color.RGBA{R: 255, A: 255}, Width: 3,
		Rect: image.Rect(10, 10, 40, 30),
	}})
	if got := out.RGBAAt(0, 0); got != (color.RGBA{R: 0, G: 0, B: 255, A: 255}) {
		t.Errorf("角上被改了：%v", got)
	}
	red := false
	for x := 10; x < 40; x++ {
		if c := out.RGBAAt(x, 10); c.R > 200 && c.G < 80 {
			red = true
			break
		}
	}
	if !red {
		t.Error("矩形上边没有画上")
	}
}

// TestRasterizeCrop checks that a crop is the picture at that part, with the
// annotations in the picture's coordinates.
func TestRasterizeCrop(t *testing.T) {
	src := bluePicture(60, 40)
	crop := image.Rect(10, 10, 40, 30)
	out := Rasterize(src, crop, nil)
	if out.Bounds() != image.Rect(0, 0, 30, 20) {
		t.Fatalf("crop = %v，want 30×20", out.Bounds())
	}
	// A rectangle at the crop's top-left is drawn there, in the result.
	out = Rasterize(src, crop, []Shape{{
		Tool: ToolRect, Color: color.RGBA{R: 255, A: 255}, Width: 2,
		Rect: image.Rect(10, 10, 40, 30),
	}})
	red := false
	for x := 0; x < 30; x++ {
		if c := out.RGBAAt(x, 0); c.R > 200 && c.G < 80 {
			red = true
			break
		}
	}
	if !red {
		t.Error("裁剪后矩形没有跟着移位")
	}
}

// TestArrowTapers checks the arrow's shape in the picture: thin at the tail,
// wider along the shaft, and widest in the head at the tip.
func TestArrowTapers(t *testing.T) {
	src := bluePicture(220, 100)
	out := Rasterize(src, src.Bounds(), []Shape{{
		Tool: ToolArrow, Color: color.RGBA{R: 255, A: 255}, Width: 3,
		From: image.Pt(10, 50), To: image.Pt(200, 50),
	}})
	// How thick the arrow is at a column: the red pixels down it.
	thick := func(x int) int {
		n := 0
		for y := 0; y < 100; y++ {
			if c := out.RGBAAt(x, y); c.R > 200 && c.B < 80 {
				n++
			}
		}
		return n
	}
	tail, mid, head := thick(20), thick(120), thick(190)
	if tail >= mid {
		t.Errorf("尾端 %d 像素粗，中段 %d，尾部没有更细", tail, mid)
	}
	if head <= mid {
		t.Errorf("箭头处 %d 像素粗，中段 %d，箭头没有更宽", head, mid)
	}
	if tail < 1 {
		t.Error("尾端没有画上")
	}
}

// TestRasterizeEllipse checks that a circle is a ring: the picture shows
// through the middle, and the ring runs through the middle of each side of
// the box it was dragged over.
func TestRasterizeEllipse(t *testing.T) {
	src := bluePicture(120, 80)
	out := Rasterize(src, src.Bounds(), []Shape{{
		Tool: ToolEllipse, Color: color.RGBA{R: 255, A: 255}, Width: 3,
		Rect: image.Rect(20, 10, 100, 70),
	}})
	red := func(x, y int) bool {
		c := out.RGBAAt(x, y)
		return c.R > 200 && c.B < 80
	}
	if red(60, 40) {
		t.Error("圆圈中间被涂实了")
	}
	for _, pt := range []image.Point{{60, 10}, {60, 70}, {20, 40}, {100, 40}} {
		if !red(pt.X, pt.Y) {
			t.Errorf("%v 处没有画上圆边", pt)
		}
	}
}

// TestRasterizeText checks that text is drawn into the picture.
func TestRasterizeText(t *testing.T) {
	src := bluePicture(120, 40)
	out := Rasterize(src, src.Bounds(), []Shape{{
		Tool: ToolText, Color: color.RGBA{R: 255, G: 255, B: 255, A: 255},
		At: image.Pt(4, 4), Text: "Ab 字", Size: 20,
	}})
	drawn := 0
	for y := 0; y < 40; y++ {
		for x := 0; x < 120; x++ {
			if c := out.RGBAAt(x, y); c.R > 200 && c.G > 200 {
				drawn++
			}
		}
	}
	if drawn < 10 {
		t.Errorf("文字只画了 %d 个像素", drawn)
	}
}

// TestRasterizeMosaic checks that a mosaic replaces the area with blocks of
// one color each, and leaves the rest alone.
func TestRasterizeMosaic(t *testing.T) {
	// A picture of two halves, so that a block has one color to take. The
	// blocks are 8 pixels wide, so the halves meet between two of them.
	src := bluePicture(40, 20)
	draw.Draw(src, image.Rect(24, 0, 40, 20), image.NewUniform(color.RGBA{R: 255, A: 255}), image.Point{}, draw.Src)
	out := Rasterize(src, src.Bounds(), []Shape{{
		Tool: ToolMosaic, Width: 8, Rect: image.Rect(0, 0, 40, 20),
	}})
	if c := out.RGBAAt(25, 10); c != (color.RGBA{R: 255, A: 255}) {
		t.Errorf("右边的块 = %v，want 红", c)
	}
	if c := out.RGBAAt(19, 10); c != (color.RGBA{R: 0, G: 0, B: 255, A: 255}) {
		t.Errorf("左边的块 = %v，want 蓝", c)
	}
}

// TestRasterizeWithinFrame checks that the offscreen render works while a
// frame is being built, as it does when the page copies a picture.
func TestRasterizeWithinFrame(t *testing.T) {
	src := bluePicture(40, 30)
	var out *image.RGBA
	built := false
	tr := ui.NewTester(func(c *ui.Context) {
		ui.Box(c).Fill()
		if !built {
			built = true
			out = Rasterize(src, src.Bounds(), []Shape{{
				Tool: ToolRect, Color: color.RGBA{R: 255, A: 255}, Width: 2,
				Rect: image.Rect(5, 5, 20, 20),
			}})
		}
	}, 60, 40)
	tr.Frame()
	if out == nil {
		t.Fatal("帧里没有渲染出标注")
	}
	if c := out.RGBAAt(0, 0); c.B != 255 {
		t.Errorf("角上 = %v，want 蓝", c)
	}
}

// TestEncode checks that the PNG is the picture's size.
func TestEncode(t *testing.T) {
	src := bluePicture(32, 18)
	data, err := Encode(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if img.Bounds() != image.Rect(0, 0, 32, 18) {
		t.Errorf("PNG = %v，want 32×18", img.Bounds())
	}
}
