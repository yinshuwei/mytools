package shots

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"

	"github.com/egoist/mygo/ui"
)

// Tool is what a drag on the picture draws.
type Tool uint8

const (
	ToolRect Tool = iota
	ToolArrow
	ToolPen
	ToolText
	ToolMosaic
)

// Names are the tools as the toolbar shows them.
var Names = []string{"矩形", "箭头", "画笔", "文字", "马赛克"}

// A Shape is one annotation, in the picture's own pixels: what is saved is
// the picture's resolution, whatever size the window shows it at.
type Shape struct {
	Tool  Tool
	Color color.RGBA
	// Width is the stroke in pixels; for a mosaic, the size of one block.
	Width float32
	// Rect is what a rectangle and a mosaic cover; From and To are the ends
	// of an arrow; Points are a pen's path.
	Rect   image.Rectangle
	From   image.Point
	To     image.Point
	Points []image.Point
	// At is where a text starts, Text what it says, Size its size in pixels.
	At   image.Point
	Text string
	Size float32
}

// Draw paints shapes on p, with the picture's top-left corner at (ox, oy),
// one of its pixels k DIPs wide, and src the picture itself, which a mosaic
// reads.
func Draw(p *ui.Painter, shapes []Shape, ox, oy, k float32, src image.Image) {
	for _, s := range shapes {
		s.draw(p, ox, oy, k, src)
	}
}

func (s Shape) draw(p *ui.Painter, ox, oy, k float32, src image.Image) {
	if k <= 0 {
		return
	}
	col := ui.RGBA(s.Color.R, s.Color.G, s.Color.B, float32(s.Color.A)/255)
	width := max(s.Width*k, 1)
	switch s.Tool {
	case ToolRect:
		p.Stroke(rectAt(s.Rect, ox, oy, k), col, 0, width)
	case ToolArrow:
		s.drawArrow(p, ox, oy, k, width, col)
	case ToolPen:
		s.drawPen(p, ox, oy, k, width, col)
	case ToolText:
		p.Text(ox+float32(s.At.X)*k, oy+float32(s.At.Y)*k, s.Text, max(s.Size*k, 1), col)
	case ToolMosaic:
		s.drawMosaic(p, ox, oy, k, src)
	}
}

func (s Shape) drawArrow(p *ui.Painter, ox, oy, k, width float32, col ui.Color) {
	x0, y0 := ox+float32(s.From.X)*k, oy+float32(s.From.Y)*k
	x1, y1 := ox+float32(s.To.X)*k, oy+float32(s.To.Y)*k
	stroke(p, x0, y0, x1, y1, width, col)
	ang := math.Atan2(float64(y1-y0), float64(x1-x0))
	head := max(10*k, 3.2*width)
	// Two strokes back from the tip, as the barbs of an arrow.
	for _, d := range []float64{ang + math.Pi - 0.55, ang + math.Pi + 0.55} {
		stroke(p, x1, y1,
			x1+float32(math.Cos(d))*head,
			y1+float32(math.Sin(d))*head, width, col)
	}
}

// stroke draws a line of any direction: Painter.Line goes only along an axis,
// so the stroke is a capsule turned to the line's angle.
func stroke(p *ui.Painter, x0, y0, x1, y1, width float32, col ui.Color) {
	dx, dy := x1-x0, y1-y0
	length := float32(math.Hypot(float64(dx), float64(dy)))
	if length < 0.5 {
		return
	}
	deg := float32(math.Atan2(float64(dy), float64(dx)) * 180 / math.Pi)
	p.Transform(ui.Rotation(deg), x0, y0, func() {
		p.Fill(ui.Rect{X: x0, Y: y0 - width/2, W: length, H: width}, col, width/2)
	})
}

func (s Shape) drawPen(p *ui.Painter, ox, oy, k, width float32, col ui.Color) {
	if len(s.Points) == 0 {
		return
	}
	// Rounded ends, and rounded joints between the segments: a disc at each
	// point fills the wedge a corner would leave.
	r := width / 2
	for i := 1; i < len(s.Points); i++ {
		a, b := s.Points[i-1], s.Points[i]
		stroke(p, ox+float32(a.X)*k, oy+float32(a.Y)*k,
			ox+float32(b.X)*k, oy+float32(b.Y)*k, width, col)
	}
	for _, pt := range s.Points {
		x, y := ox+float32(pt.X)*k, oy+float32(pt.Y)*k
		p.Fill(ui.Rect{X: x - r, Y: y - r, W: 2 * r, H: 2 * r}, col, r)
	}
}

func (s Shape) drawMosaic(p *ui.Painter, ox, oy, k float32, src image.Image) {
	if src == nil {
		return
	}
	block := int(max(s.Width, 4))
	area := s.Rect.Intersect(src.Bounds())
	for y := area.Min.Y; y < area.Max.Y; y += block {
		for x := area.Min.X; x < area.Max.X; x += block {
			x2, y2 := min(x+block, area.Max.X), min(y+block, area.Max.Y)
			cr, cg, cb, ca := src.At((x+x2)/2, (y+y2)/2).RGBA()
			p.Fill(ui.Rect{
				X: ox + float32(x)*k,
				Y: oy + float32(y)*k,
				W: float32(x2-x) * k,
				H: float32(y2-y) * k,
			}, ui.RGBA(uint8(cr>>8), uint8(cg>>8), uint8(cb>>8), float32(ca)/65535), 0)
		}
	}
}

// rectAt places a picture rectangle at (ox, oy), k DIPs to a pixel.
func rectAt(r image.Rectangle, ox, oy, k float32) ui.Rect {
	return ui.Rect{
		X: ox + float32(r.Min.X)*k,
		Y: oy + float32(r.Min.Y)*k,
		W: float32(r.Dx()) * k,
		H: float32(r.Dy()) * k,
	}
}

// Rasterize draws the shapes over the cropped part of src, at the picture's
// own pixels, and returns the result.
func Rasterize(src *image.RGBA, crop image.Rectangle, shapes []Shape) *image.RGBA {
	crop = crop.Intersect(src.Bounds())
	if crop.Empty() {
		return image.NewRGBA(image.Rect(0, 0, 1, 1))
	}
	out := image.NewRGBA(image.Rect(0, 0, crop.Dx(), crop.Dy()))
	draw.Draw(out, out.Bounds(), src, crop.Min, draw.Src)
	if len(shapes) == 0 {
		return out
	}
	// The annotations are drawn by the same painter that shows them, into a
	// picture of their own, which is laid over the screenshot: what is saved
	// is exactly what the page showed, at the picture's resolution.
	layer := ui.Render(func(c *ui.Context) {
		c.SetTheme(&ui.Theme{Background: ui.Transparent})
		ui.Box(c).Fill().Draw(func(p *ui.Painter, r ui.Rect) {
			Draw(p, shapes, float32(-crop.Min.X), float32(-crop.Min.Y), 1, src)
		})
	}, crop.Dx(), crop.Dy(), 1)
	draw.Draw(out, out.Bounds(), layer, image.Point{}, draw.Over)
	return out
}

// Encode renders the picture with its annotations as a PNG.
func Encode(src *image.RGBA, shapes []Shape) ([]byte, error) {
	out := Rasterize(src, src.Bounds(), shapes)
	var buf bytes.Buffer
	if err := png.Encode(&buf, out); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
