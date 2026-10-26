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
