package shots

import (
	"strconv"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/yinshuwei/mytools/internal/capture"
)

// overlay is one display's window: its frozen picture, the selection on it,
// and the hint that says what Enter does.
type overlay struct {
	s     *Service
	frame capture.Frame
	bmp   *ui.Bitmap
	win   *mygo.Window

	// The selection is in the content area's DIPs, from its top-left.
	sel              ui.Rect
	anchorX, anchorY float32
	dragging         bool
	px, py           float32
	pointer          bool

	// size is the content area, as the last frame drew it.
	size ui.Rect
}

func newOverlay(s *Service, f capture.Frame) *overlay {
	return &overlay{s: s, frame: f, bmp: ui.NewBitmap(f.Image)}
}

// create makes the display's window, hidden: the service shows all of them
// at once, once every one is ready.
func (o *overlay) create() {
	d := o.frame.Display
	o.win = mygo.NewWindow(mygo.WindowOptions{
		Title:             "截图",
		X:                 d.X,
		Y:                 d.Y,
		Width:             d.Width,
		Height:            d.Height,
		UseContentSize:    true,
		Hidden:            true,
		Frameless:         true,
		BackgroundColor:   "#000000",
		AlwaysOnTop:       true,
		SkipTaskbar:       true,
		DisableResize:     true,
		DisableMove:       true,
		DisableMinimize:   true,
		DisableMaximize:   true,
		DisableFullScreen: true,
		DisableClose:      true,
		DisableShadow:     true,
		Content:           ui.View(o.view),
	})
	o.win.OnClose(func(e *mygo.CloseEvent) {
		e.PreventDefault()
		o.s.cancel()
	})
}

func (o *overlay) view(c *ui.Context) {
	c.OnShortcut(0, ui.KeyEscape, func() { o.s.cancel() })
	c.OnShortcut(0, ui.KeyEnter, func() { o.confirm() })
	ui.Box(c).FillWidth().FillHeight().Cursor(ui.CursorCrosshair).
		HandleInput(o.input).Draw(o.draw)
}

// confirm hands the chosen pixels to the toolbox, which edits and annotates
// them where a text field and a toolbar are at home.
func (o *overlay) confirm() {
	if o.hasSelection() {
		o.s.editOverlay(o)
	}
}

func (o *overlay) hasSelection() bool {
	return o.sel.W > 0 && o.sel.H > 0
}

// input takes the pointer: a drag selects, a right click leaves.
func (o *overlay) input(ev ui.InputEvent) bool {
	switch ev.Kind {
	case ui.InputPointerDown:
		switch ev.Button {
		case 1:
			o.s.cancel()
			return true
		case 0:
		default:
			return false
		}
		o.dragging = true
		o.anchorX, o.anchorY = ev.X, ev.Y
		o.sel = ui.Rect{X: ev.X, Y: ev.Y}
		o.px, o.py, o.pointer = ev.X, ev.Y, true
		return true
	case ui.InputPointerMove:
		o.px, o.py, o.pointer = ev.X, ev.Y, true
		if o.dragging {
			o.sel = clampSelection(rectFromDrag(o.anchorX, o.anchorY, ev.X, ev.Y), o.size.W, o.size.H)
		}
		return true
	case ui.InputPointerUp:
		if ev.Button != 0 || !o.dragging {
			return false
		}
		o.dragging = false
		o.sel = clampSelection(rectFromDrag(o.anchorX, o.anchorY, ev.X, ev.Y), o.size.W, o.size.H)
		if o.sel.W < minSelection || o.sel.H < minSelection {
			o.sel = ui.Rect{}
		}
		return true
	}
	return false
}

// draw paints the frozen display, darkens what is not selected, and says
// what Enter does.
func (o *overlay) draw(p *ui.Painter, r ui.Rect) {
	o.size = ui.Rect{W: r.W, H: r.H}
	p.ImageRegion(o.bmp, ui.Rect{}, r, ui.ImageOptions{})

	if o.hasSelection() {
		o.dim(p, r)
		o.drawSelection(p, r)
		o.drawSize(p, r)
	}
	o.drawHint(p, r)
	if o.dragging {
		o.drawPointer(p, r)
	}
}

func (o *overlay) dim(p *ui.Painter, r ui.Rect) {
	sel := o.sel
	dark := ui.RGBA(0, 0, 0, 0.45)
	p.Fill(ui.Rect{X: r.X, Y: r.Y, W: r.W, H: sel.Y}, dark, 0)
	p.Fill(ui.Rect{X: r.X, Y: r.Y + sel.Y + sel.H, W: r.W, H: r.H - sel.Y - sel.H}, dark, 0)
	p.Fill(ui.Rect{X: r.X, Y: r.Y + sel.Y, W: sel.X, H: sel.H}, dark, 0)
	p.Fill(ui.Rect{X: r.X + sel.X + sel.W, Y: r.Y + sel.Y, W: r.W - sel.X - sel.W, H: sel.H}, dark, 0)
}

func (o *overlay) drawSelection(p *ui.Painter, r ui.Rect) {
	sel := ui.Rect{X: r.X + o.sel.X, Y: r.Y + o.sel.Y, W: o.sel.W, H: o.sel.H}
	p.Stroke(sel, accent, 0, 2)
}

func (o *overlay) drawSize(p *ui.Painter, r ui.Rect) {
	w, h := pixelSize(o.sel, o.size, o.frame.Image.Bounds())
	label := strconv.Itoa(w) + " × " + strconv.Itoa(h)
	tw, th := p.MeasureText(0, ui.Span{Text: label, Size: 12})
	const padX, padY = 8, 5
	boxW, boxH := tw+2*padX, th+2*padY
	x := o.sel.X + 8
	y := o.sel.Y + 8
	if x+boxW > r.W-8 {
		x = r.W - 8 - boxW
	}
	if y+boxH > o.sel.Y+o.sel.H {
		y = o.sel.Y - boxH - 6
	}
	if y < 6 {
		y = o.sel.Y + 8
	}
	box := ui.Rect{X: r.X + x, Y: r.Y + y, W: boxW, H: boxH}
	p.Fill(box, ui.RGBA(0, 0, 0, 0.76), 4)
	p.Text(box.X+padX, box.Y+padY, label, 12, ui.RGB(255, 255, 255))
}

func (o *overlay) drawHint(p *ui.Painter, r ui.Rect) {
	text := "拖动鼠标框选 · Esc 取消"
	if o.hasSelection() {
		text = "Enter 进入编辑与标注 · Esc 取消"
	}
	tw, th := p.MeasureText(0, ui.Span{Text: text, Size: 13})
	const padX, padY = 14, 8
	w, h := tw+2*padX, th+2*padY
	x := (r.W - w) / 2
	if x < 8 {
		x = 8
	}
	y := float32(28)
	box := ui.Rect{X: r.X + x, Y: r.Y + y, W: w, H: h}
	p.Fill(box, ui.RGBA(18, 18, 18, 0.78), 8)
	p.Text(box.X+padX, box.Y+padY, text, 13, ui.RGB(255, 255, 255))
}

func (o *overlay) drawPointer(p *ui.Painter, r ui.Rect) {
	line := ui.RGBA(255, 255, 255, 0.32)
	p.Fill(ui.Rect{X: r.X + o.px, Y: r.Y, W: 1, H: r.H}, line, 0)
	p.Fill(ui.Rect{X: r.X, Y: r.Y + o.py, W: r.W, H: 1}, line, 0)
}
