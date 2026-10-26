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
	ToolSelect Tool = iota
	ToolRect
	ToolEllipse
	ToolArrow
	ToolPen
	ToolText
	ToolMosaic
)

// Names are the tools as the toolbar shows them.
var Names = []string{"选择", "矩形", "圆圈", "箭头", "画笔", "文字", "马赛克"}

// A Shape is one annotation, in the picture's own pixels: what is saved is
// the picture's resolution, whatever size the window shows it at.
type Shape struct {
	Tool  Tool
	Color color.RGBA
	// Width is the stroke in pixels; for a mosaic, the size of one block.
	Width float32
	// Rect is what a rectangle, a circle and a mosaic cover; From and To
	// are the ends of an arrow; Points are a pen's path.
	Rect   image.Rectangle
	From   image.Point
	To     image.Point
	Points []image.Point
	// At is where a text starts, Text what it says, Size its size in pixels.
	At   image.Point
	Text string
	Size float32
}

// Bounds is the box a shape covers, in the picture's pixels, stroke and all:
// the box that follows it as it is moved.
func (s Shape) Bounds() image.Rectangle {
	half := int(s.Width / 2)
	switch s.Tool {
	case ToolRect, ToolEllipse, ToolMosaic:
		return s.Rect.Inset(-half)
	case ToolArrow:
		// The head is wider than the stroke, so the box takes its width.
		return image.Rect(
			min(s.From.X, s.To.X), min(s.From.Y, s.To.Y),
			max(s.From.X, s.To.X), max(s.From.Y, s.To.Y),
		).Inset(-int(arrowHeadW(s.Width) / 2))
	case ToolPen:
		if len(s.Points) == 0 {
			return image.Rectangle{}
		}
		r := image.Rect(s.Points[0].X, s.Points[0].Y, s.Points[0].X, s.Points[0].Y)
		for _, pt := range s.Points[1:] {
			r = r.Union(image.Rect(pt.X, pt.Y, pt.X, pt.Y))
		}
		return r.Inset(-half)
	case ToolText:
		return image.Rect(s.At.X, s.At.Y,
			s.At.X+int(textWidth(s.Text, s.Size)), s.At.Y+int(s.Size*1.25))
	}
	return image.Rectangle{}
}

// Hit says whether pt, a pixel of the picture, is on the shape, with tol
// pixels of slack: a line is taken by its stroke, not by the box around it.
func (s Shape) Hit(pt image.Point, tol float32) bool {
	reach := float64(tol + s.Width/2)
	switch s.Tool {
	case ToolRect, ToolMosaic:
		// The inside counts as well: a rectangle is easier to take by
		// what it frames than by its 2-pixel outline.
		return pt.In(s.Rect.Inset(-int(tol)))
	case ToolEllipse:
		// A ring is taken by what it circles, as a rectangle is.
		rx, ry := float64(s.Rect.Dx())/2+float64(tol), float64(s.Rect.Dy())/2+float64(tol)
		if rx <= 0 || ry <= 0 {
			return false
		}
		dx := float64(pt.X) - (float64(s.Rect.Min.X) + float64(s.Rect.Dx())/2)
		dy := float64(pt.Y) - (float64(s.Rect.Min.Y) + float64(s.Rect.Dy())/2)
		return (dx*dx)/(rx*rx)+(dy*dy)/(ry*ry) <= 1
	case ToolArrow:
		if distToSegment(pt, s.From, s.To) <= reach {
			return true
		}
		// The head is a triangle at the tip, wider than the stroke.
		return dist(pt, s.To) <= float64(tol+arrowHeadW(s.Width)/2)
	case ToolPen:
		if len(s.Points) == 1 {
			return dist(pt, s.Points[0]) <= reach
		}
		for i := 1; i < len(s.Points); i++ {
			if distToSegment(pt, s.Points[i-1], s.Points[i]) <= reach {
				return true
			}
		}
		return false
	case ToolText:
		return pt.In(s.Bounds().Inset(-int(tol)))
	}
	return false
}

// Moved returns the shape shifted by dx, dy pixels of the picture.
func (s Shape) Moved(dx, dy int) Shape {
	d := image.Pt(dx, dy)
	switch s.Tool {
	case ToolRect, ToolEllipse, ToolMosaic:
		s.Rect = s.Rect.Add(d)
	case ToolArrow:
		s.From, s.To = s.From.Add(d), s.To.Add(d)
	case ToolText:
		s.At = s.At.Add(d)
	case ToolPen:
		// A pen's path is its own: the shape it was moved from keeps the
		// one it had, as undo goes back to it.
		pts := make([]image.Point, len(s.Points))
		for i, pt := range s.Points {
			pts[i] = pt.Add(d)
		}
		s.Points = pts
	}
	return s
}

// textWidth estimates how wide a line of text runs, for a pointer to take
// it: wide letters of a Latin alphabet are about half an em, and a CJK
// character a whole one.
func textWidth(text string, size float32) float32 {
	var w float32
	for _, r := range text {
		if r >= 0x2e80 {
			w += size
		} else {
			w += size * 0.55
		}
	}
	return w
}

// distToSegment is how far pt is from the line from a to b.
func distToSegment(pt, a, b image.Point) float64 {
	ax, ay := float64(a.X), float64(a.Y)
	bx, by := float64(b.X), float64(b.Y)
	px, py := float64(pt.X), float64(pt.Y)
	dx, dy := bx-ax, by-ay
	len2 := dx*dx + dy*dy
	t := 0.0
	if len2 > 0 {
		t = min(max(((px-ax)*dx+(py-ay)*dy)/len2, 0), 1)
	}
	return math.Hypot(px-(ax+t*dx), py-(ay+t*dy))
}

func dist(a, b image.Point) float64 {
	return math.Hypot(float64(a.X-b.X), float64(a.Y-b.Y))
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
	case ToolEllipse:
		s.drawEllipse(p, ox, oy, k, width, col)
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

// drawArrow paints an arrow that swells from a thin tail to a wide head, as
// the ones a phone's screenshot tool draws: the shaft thickens along its
// length, and the head is a solid triangle.
func (s Shape) drawArrow(p *ui.Painter, ox, oy, k, width float32, col ui.Color) {
	x0, y0 := ox+float32(s.From.X)*k, oy+float32(s.From.Y)*k
	x1, y1 := ox+float32(s.To.X)*k, oy+float32(s.To.Y)*k
	dx, dy := x1-x0, y1-y0
	length := float32(math.Hypot(float64(dx), float64(dy)))
	if length < 2 {
		return
	}
	deg := float32(math.Atan2(float64(dy), float64(dx)) * 180 / math.Pi)

	w := max(width, 1)
	// The head is a triangle a few strokes wide and a little longer than
	// that; the shaft runs into it, so that the two read as one.
	headW := min(arrowHeadW(w), length)
	head := min(headW*1.3, length*0.6)
	tail, neck := max(w*0.42, 1), max(headW*0.36, 1.2)
	shaft := length - head*0.75

	p.Transform(ui.Rotation(deg), x0, y0, func() {
		taper(p, col, x0, y0, 0, shaft, tail, neck, 1)
		taper(p, col, x0, y0, shaft, length, headW, 0, 0.5)
		// The tail is rounded, thin as it is.
		p.Fill(ui.Rect{X: x0 - tail/2, Y: y0 - tail/2, W: tail, H: tail}, col, tail/2)
	})
}

// arrowHeadW is how wide the head of an arrow whose stroke is w DIPs is.
func arrowHeadW(w float32) float32 { return max(w*4.5, 12) }

// taper fills a stretch of the axis that starts at (x0, y0), turned to the
// shaft, from width w0 at t0 to w1 at t1. The painter draws rectangles, so a
// wedge is a stack of them: each piece is a hair wider than the last, and
// they overlap a little, so that the sides come out straight.
func taper(p *ui.Painter, col ui.Color, x0, y0, t0, t1, w0, w1, step float32) {
	if t1 <= t0 {
		return
	}
	steps := max(int((t1-t0)/step), 1)
	for i := 0; i < steps; i++ {
		a := t0 + (t1-t0)*float32(i)/float32(steps)
		b := t0 + (t1-t0)*float32(i+1)/float32(steps)
		w := max(w0+(w1-w0)*(float32(i)+0.5)/float32(steps), 0.6)
		p.Fill(ui.Rect{X: x0 + a, Y: y0 - w/2, W: b - a + 0.75, H: w}, col, 0)
	}
}

// drawEllipse strokes the ring inside the shape's box: a circle, or an oval
// as wide as the box is. The painter has no curves of its own, so the ring is
// a run of short segments.
func (s Shape) drawEllipse(p *ui.Painter, ox, oy, k, width float32, col ui.Color) {
	r := rectAt(s.Rect, ox, oy, k)
	cx, cy := r.X+r.W/2, r.Y+r.H/2
	rx, ry := r.W/2, r.H/2
	if rx < 0.75 || ry < 0.75 {
		return
	}
	// Pieces of about a DIP and a half: enough that no corner of the ring
	// shows, and few enough to keep a drag that is making one smooth.
	n := min(max(int((rx+ry)*math.Pi/0.75), 36), 720)
	prevX, prevY := cx+rx, cy
	for i := 1; i <= n; i++ {
		a := 2 * math.Pi * float64(i) / float64(n)
		x, y := cx+rx*float32(math.Cos(a)), cy+ry*float32(math.Sin(a))
		bar(p, prevX, prevY, x, y, width, col)
		prevX, prevY = x, y
	}
}

// bar draws a straight piece of a ring: a rectangle turned to the line, with
// flat ends, so that the pieces of a curve join into a smooth edge.
func bar(p *ui.Painter, x0, y0, x1, y1, width float32, col ui.Color) {
	dx, dy := x1-x0, y1-y0
	length := float32(math.Hypot(float64(dx), float64(dy)))
	if length < 0.01 {
		return
	}
	deg := float32(math.Atan2(float64(dy), float64(dx)) * 180 / math.Pi)
	p.Transform(ui.Rotation(deg), x0, y0, func() {
		// A hair of overlap at both ends, so that no seam shows at a joint.
		p.Fill(ui.Rect{X: x0 - 0.4, Y: y0 - width/2, W: length + 0.8, H: width}, col, 0)
	})
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
