package tools

import (
	"fmt"
	"image"
	"image/color"
	"runtime"
	"strconv"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/yinshuwei/mytools/internal/shots"
	"github.com/yinshuwei/mytools/internal/widgets"
)

// shotColors are the pens the page offers, in the order of the swatches.
var shotColors = []struct {
	name string
	col  color.RGBA
}{
	{"红", color.RGBA{R: 0xff, G: 0x3b, B: 0x30, A: 0xff}},
	{"橙", color.RGBA{R: 0xff, G: 0x95, B: 0x00, A: 0xff}},
	{"黄", color.RGBA{R: 0xff, G: 0xcc, B: 0x00, A: 0xff}},
	{"绿", color.RGBA{R: 0x34, G: 0xc7, B: 0x59, A: 0xff}},
	{"蓝", color.RGBA{R: 0x0a, G: 0x84, B: 0xff, A: 0xff}},
	{"紫", color.RGBA{R: 0xbf, G: 0x5a, B: 0xf2, A: 0xff}},
	{"白", color.RGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}},
	{"黑", color.RGBA{R: 0x00, G: 0x00, B: 0x00, A: 0xff}},
}

// Screenshot starts a screenshot, then edits and annotates the picture the
// overlay hands back: the overlay is only for choosing the region, so that a
// toolbar, a text field and the keyboard are at home here.
type Screenshot struct {
	// Shot starts a screenshot. The app puts it in place once it has a
	// window to hide and capture.
	Shot func()
	// Hotkey is how the global shortcut is written for people, e.g.
	// Ctrl+Alt+A.
	Hotkey string
	// Problem is why screenshots cannot be taken on this machine or
	// session.
	Problem string
	// HotkeyProblem is why the global shortcut could not be registered;
	// the button still works.
	HotkeyProblem string

	win *mygo.Window

	img   *image.RGBA
	bmp   *ui.Bitmap
	scale float64 // screen pixels per DIP, for strokes and text

	// shapes are the annotations, and past and future are the states undo
	// and redo go through: a move changes a shape where it lies, so the
	// history keeps whole states rather than the shapes themselves.
	shapes []shots.Shape
	past   [][]shots.Shape
	future [][]shots.Shape

	tool  shots.Tool
	color int     // index into shotColors
	width float64 // stroke, in DIPs

	drawing bool
	start   image.Point
	cur     image.Point
	pen     []image.Point

	// The shape the pointer took, and where it was taken from: dragging it
	// moves it.
	sel      int
	moving   bool
	moved    bool
	moveFrom image.Point
	moveBase shots.Shape

	textAt    image.Point
	text      string
	typing    bool
	focusText bool

	// k is how many DIPs one picture pixel takes: one screen pixel, so on a
	// HiDPI screen a fraction of a DIP.
	k float32

	// said reports what a save came to, from the goroutine that ran the
	// dialog.
	said chan string
}

func (*Screenshot) ID() string   { return "shot" }
func (*Screenshot) Name() string { return "截图" }
func (*Screenshot) Desc() string { return "框选屏幕，在页面里标注、复制或保存" }

// MaxWidth keeps the page before a picture a column, as the other pages of
// controls are; the editor itself takes the whole width, so that a wide
// screenshot has the room it needs.
func (s *Screenshot) MaxWidth() float32 {
	if s.img != nil {
		return 0
	}
	return 720
}

// FillsHeight gives the picture the whole room under the toolbar.
func (*Screenshot) FillsHeight() bool { return true }

// SetWindow is what WindowAware asks of a tool acting on the window: the
// save dialog is a sheet of it.
func (s *Screenshot) SetWindow(win *mygo.Window) { s.win = win }

// SetImage takes the picture the overlay chose, with the display's scale in
// pixels per DIP, and starts the annotations afresh.
func (s *Screenshot) SetImage(img *image.RGBA, scale float64) {
	if img == nil || img.Bounds().Empty() {
		return
	}
	if scale <= 0 {
		scale = 1
	}
	s.img, s.bmp, s.scale = img, ui.NewBitmap(img), scale
	if s.width <= 0 {
		s.width = 3
	}
	s.shapes, s.past, s.future = nil, nil, nil
	s.sel = -1
	s.drawing, s.typing, s.text = false, false, ""
	s.textAt = image.Point{}
	// One picture pixel is one screen pixel: on a HiDPI screen that is a
	// fraction of a DIP, so that nothing is scaled twice.
	s.k = float32(1 / scale)
	s.focusText = false
}

func (s *Screenshot) View(c *ui.Context) {
	t := c.Theme()
	if s.said == nil {
		s.said = make(chan string, 4)
	}
	widgets.Drain(s.said, func(msg string) { c.Toast(msg) })
	if s.img == nil {
		s.landing(c, t)
		return
	}
	s.shortcuts(c)
	ui.Column(c).Grow(1).FillWidth().Gap(10).Children(func() {
		// The controls stay in a column a hand can reach, as on the other
		// pages; the picture below them takes the whole width.
		ui.Column(c).FillWidth().MaxWidth(PageWidth).Gap(10).Children(func() {
			s.toolbar(c, t)
		})
		s.canvasView(c)
		s.status(c)
	})
}

// landing is the page before a picture comes: the button, the shortcut, and
// what will happen.
func (s *Screenshot) landing(c *ui.Context, t *ui.Theme) {
	ui.Column(c).Gap(12).Children(func() {
		ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
			start := ui.PrimaryButton(c, "开始截图")
			if s.Shot == nil || s.Problem != "" || s.HotkeyProblem != "" {
				start = start.Disabled(true)
			}
			if start.Clicked() && s.Shot != nil {
				s.Shot()
			}
			if s.Hotkey != "" {
				widgets.Hint(c, "全局快捷键 "+s.Hotkey)
			}
		})
		if s.Problem != "" {
			ui.Text(c, s.Problem).FontSize(widgets.Scaled(12)).TextColor(t.Danger)
		}
		if s.HotkeyProblem != "" {
			ui.Text(c, s.HotkeyProblem).FontSize(widgets.Scaled(12)).TextColor(t.Danger)
		}
		ui.Divider(c)
		ui.Text(c, "拖动鼠标框选一块区域，然后按 Enter。").
			FontSize(widgets.Scaled(13))
		ui.Text(c, "图片会回到这里，可以画矩形、箭头、画笔、文字和马赛克，再复制或保存。").
			FontSize(widgets.Scaled(12)).TextColor(t.TextMuted)
		if runtime.GOOS == "darwin" {
			ui.Text(c, "第一次使用需要在系统设置 → 隐私与安全性 → 屏幕录制里允许本应用，然后重新启动它。").
				FontSize(widgets.Scaled(12)).TextColor(t.TextMuted)
		}
	})
}

// shortcuts are the keys of the editor: undo, redo, copy and save, and Esc
// to drop what is being drawn or the whole picture.
func (s *Screenshot) shortcuts(c *ui.Context) {
	if !s.typing {
		if c.Shortcut(ui.Cmd, ui.KeyZ) {
			s.undo()
		}
		if c.Shortcut(ui.Cmd|ui.Shift, ui.KeyZ) {
			s.redoStep()
		}
	}
	if s.typing {
		if c.Shortcut(0, ui.KeyEscape) {
			s.typing, s.text = false, ""
		}
		return
	}
	if c.Shortcut(ui.Cmd, ui.KeyC) {
		s.copyImage(c)
	}
	if c.Shortcut(ui.Cmd, ui.KeyS) {
		s.saveImage(c)
	}
	if c.Shortcut(0, ui.KeyEscape) {
		if s.drawing {
			s.drawing = false
		} else if s.sel >= 0 {
			s.sel = -1
		} else {
			s.clear()
		}
	}
	if s.tool == shots.ToolSelect && s.sel >= 0 &&
		(c.Shortcut(0, ui.KeyDelete) || c.Shortcut(0, ui.KeyBackspace)) {
		s.removeSel()
	}
}

func (s *Screenshot) toolbar(c *ui.Context, t *ui.Theme) {
	ui.Row(c).Gap(8).Wrap().AlignItems(ui.Center).Children(func() {
		tool := int(s.tool)
		if ui.Segmented(c.Key("tool"), &tool, shots.Names...).Changed() {
			s.tool = shots.Tool(tool)
			s.typing = false
			if s.tool != shots.ToolSelect {
				s.sel = -1
			}
		}
		ui.Spacer(c)
		if ui.Button(c.Key("undo"), "撤销").Disabled(len(s.past) == 0).
			Tooltip("撤销上一步（" + widgets.ShortcutHint(0, "Z") + "）").Clicked() {
			s.undo()
		}
		if ui.Button(c.Key("redo"), "重做").Disabled(len(s.future) == 0).
			Tooltip("重做（" + widgets.ShortcutHint(ui.Shift, "Z") + "）").Clicked() {
			s.redoStep()
		}
		again := ui.Button(c.Key("again"), "重新截图").Tooltip("隐藏窗口，重新框选")
		if again.Clicked() && s.Shot != nil {
			s.Shot()
		}
		if ui.Button(c.Key("drop"), "关闭").Tooltip("放下这张图，不再编辑").Clicked() {
			s.clear()
		}
	})
	ui.Row(c).Gap(8).Wrap().AlignItems(ui.Center).Children(func() {
		for i, pen := range shotColors {
			sw := ui.Box(c.Key("pen"+strconv.Itoa(i))).
				Size(widgets.Scaled(18), widgets.Scaled(18)).Radius(widgets.Scaled(9)).
				Background(ui.RGB(pen.col.R, pen.col.G, pen.col.B)).
				Cursor(ui.CursorPointer).Tooltip("颜色：" + pen.name)
			if i == s.color {
				sw.Border(widgets.Scaled(2), t.Text)
			} else {
				sw.Border(1, t.Border)
			}
			if sw.Clicked() {
				s.color = i
			}
		}
		widgets.Hint(c, "线宽")
		ui.Slider(c.Key("width"), &s.width, 1, 10).Width(widgets.Scaled(90)).
			Label("线宽").Tooltip("画出来有多粗，以屏幕上的 DIP 计")
		ui.Spacer(c)
		if ui.PrimaryButton(c.Key("copy"), "复制").
			Tooltip("复制标注后的图片（" + widgets.ShortcutHint(0, "C") + "）").Clicked() {
			s.copyImage(c)
		}
		if ui.Button(c.Key("save"), "保存…").
			Tooltip("保存为 PNG（" + widgets.ShortcutHint(0, "S") + "）").Clicked() {
			s.saveImage(c)
		}
	})
}

// textEditor floats where the text is being typed, over the picture: a field
// in the page would push the picture down as it came and up as it went, and
// the picture is what the eye is on.
func (s *Screenshot) textEditor(c *ui.Context, t *ui.Theme) {
	r, ok := s.popRect()
	if !ok {
		return
	}
	ui.Box(c.Key("textpop")).Absolute().Left(r.X).Top(r.Y).Width(r.W).
		Background(t.Surface).Border(1, t.Border).Radius(widgets.Scaled(6)).
		Shadow(0, widgets.Scaled(3), widgets.Scaled(12), 0, ui.RGBA(0, 0, 0, 0.35)).
		Padding(widgets.Scaled(4)).
		Children(func() {
			box := ui.TextInput(c.Key("text"), &s.text).
				Placeholder("输入文字，Enter 放下").
				Tooltip("Enter 把文字放在这里，Esc 取消")
			if s.focusText {
				s.focusText = false
				box.Focus()
			}
			if box.Submitted() {
				s.commitText()
			}
		})
}

// popRect is where the text field floats, in DIPs from the picture's corner:
// under the text where there is room, over it where there is not, and never
// off the picture.
func (s *Screenshot) popRect() (ui.Rect, bool) {
	if !s.typing || s.img == nil || s.k <= 0 {
		return ui.Rect{}, false
	}
	w := float32(s.img.Bounds().Dx()) * s.k
	h := float32(s.img.Bounds().Dy()) * s.k
	fw, fh := widgets.Scaled(220), widgets.Scaled(40)
	x := float32(s.textAt.X) * s.k
	y := float32(s.textAt.Y)*s.k + widgets.Scaled(22)
	if y+fh > h {
		y = float32(s.textAt.Y)*s.k - fh - widgets.Scaled(6)
	}
	return ui.Rect{
		X: min(max(x, 0), max(w-fw, 0)),
		Y: min(max(y, 0), max(h-fh, 0)),
		W: fw, H: fh,
	}, true
}

// canvasView shows the picture at its own size, in a viewport that scrolls
// both ways: a screenshot is looked at pixel for pixel, not fitted. The
// viewport is a surface of its own, outlined and tinted, so that the picture
// and the room around it are told apart from the page.
func (s *Screenshot) canvasView(c *ui.Context) {
	if s.img == nil || s.k <= 0 {
		return
	}
	t := c.Theme()
	cursor := ui.CursorCrosshair
	switch s.tool {
	case shots.ToolText:
		cursor = ui.CursorText
	case shots.ToolSelect:
		cursor = ui.CursorMove
	}
	b := s.img.Bounds()
	w := float32(b.Dx()) * s.k
	h := float32(b.Dy()) * s.k
	ui.ScrollBoth(c.Key("scroll")).FillWidth().Grow(1).
		Background(t.SurfaceHover).Border(1, t.Border).Radius(widgets.Scaled(8)).
		Children(func() {
			ui.Column(c).Padding(widgets.Scaled(12)).Children(func() {
				ui.Box(c.Key("canvas")).Width(w).Height(h).Shrink(0).
					Label("画布").Cursor(cursor).HandleInput(s.input).Draw(s.draw).
					Children(func() {
						if s.typing {
							s.textEditor(c, t)
						}
					})
			})
		})
}

func (s *Screenshot) status(c *ui.Context) {
	ui.Row(c).Gap(12).Wrap().AlignItems(ui.Center).Children(func() {
		widgets.Hint(c, fmt.Sprintf("%d × %d 像素", s.img.Bounds().Dx(), s.img.Bounds().Dy()))
		widgets.Hint(c, "拖动绘制 · 选择工具拖动标注可移动，Delete 删除 · "+
			widgets.ShortcutHint(0, "Z")+" 撤销 · "+
			widgets.ShortcutHint(0, "C")+" 复制 · "+widgets.ShortcutHint(0, "S")+" 保存")
	})
}

// draw paints the picture at its own size, the annotations over it, and the
// shape being drawn.
func (s *Screenshot) draw(p *ui.Painter, r ui.Rect) {
	if s.img == nil || s.k <= 0 {
		return
	}
	p.Image(s.bmp, r, ui.Contain)
	shots.Draw(p, s.shapes, r.X, r.Y, s.k, s.img)
	if sh, ok := s.pendingShape(); ok {
		shots.Draw(p, []shots.Shape{sh}, r.X, r.Y, s.k, s.img)
	}
	// The shape the pointer took is ringed, so that a drag has something to
	// hold on to; what is saved has no ring.
	if s.tool == shots.ToolSelect && s.sel >= 0 && s.sel < len(s.shapes) {
		b := s.shapes[s.sel].Bounds()
		ring := ui.Rect{
			X: r.X + float32(b.Min.X)*s.k, Y: r.Y + float32(b.Min.Y)*s.k,
			W: float32(b.Dx()) * s.k, H: float32(b.Dy()) * s.k,
		}
		p.Stroke(ring, ui.RGBA(0, 0, 0, 0.45), 0, 3)
		p.Stroke(ring, ui.RGBA(255, 255, 255, 0.9), 0, 1)
	}
	p.Stroke(r, ui.RGBA(255, 255, 255, 0.25), 0, 1)
}

// input draws with the pointer: a drag makes a shape, a click of the text
// tool says where the text goes.
func (s *Screenshot) input(ev ui.InputEvent) bool {
	// The field floats over the picture: a press there is a press on the
	// field, not on the picture under it.
	if r, ok := s.popRect(); ok &&
		ev.X >= r.X && ev.X <= r.X+r.W && ev.Y >= r.Y && ev.Y <= r.Y+r.H {
		return false
	}
	switch ev.Kind {
	case ui.InputPointerDown:
		if ev.Button != 0 {
			return false
		}
		pt, ok := s.imageAt(ev.X, ev.Y)
		if !ok {
			s.sel = -1
			return false
		}
		switch s.tool {
		case shots.ToolSelect:
			s.grab(pt)
			return true
		case shots.ToolText:
			if s.typing {
				s.commitText()
			}
			s.textAt, s.text, s.typing, s.focusText = pt, "", true, true
			return true
		}
		s.drawing = true
		s.start, s.cur = pt, pt
		s.pen = []image.Point{pt}
		return true
	case ui.InputPointerMove:
		if s.moving {
			s.dragSelection(ev.X, ev.Y)
			return true
		}
		if !s.drawing {
			return false
		}
		s.cur = s.clampImage(ev.X, ev.Y)
		if s.tool == shots.ToolPen {
			if last := s.pen[len(s.pen)-1]; last != s.cur {
				s.pen = append(s.pen, s.cur)
			}
		}
		return true
	case ui.InputPointerUp:
		if ev.Button != 0 {
			return false
		}
		if s.moving {
			s.moving = false
			return true
		}
		if !s.drawing {
			return false
		}
		s.cur = s.clampImage(ev.X, ev.Y)
		if s.tool == shots.ToolPen {
			s.pen = append(s.pen, s.cur)
		}
		// The shape is taken before the drag ends: a shape is only pending
		// while it is being drawn.
		if sh, ok := s.pendingShape(); ok {
			s.commit(sh)
		}
		s.drawing = false
		return true
	}
	return false
}

// pendingShape is the shape being drawn, or the text being typed: what the
// canvas shows as it is made, and what a release commits.
func (s *Screenshot) pendingShape() (shots.Shape, bool) {
	col := shotColors[s.color].col
	width := float32(s.width) * float32(s.scale)
	switch s.tool {
	case shots.ToolRect, shots.ToolMosaic:
		if !s.drawing {
			return shots.Shape{}, false
		}
		rect := image.Rect(
			min(s.start.X, s.cur.X), min(s.start.Y, s.cur.Y),
			max(s.start.X, s.cur.X), max(s.start.Y, s.cur.Y),
		).Intersect(s.img.Bounds())
		if rect.Empty() {
			return shots.Shape{}, false
		}
		if s.tool == shots.ToolMosaic {
			// A block worth seeing, whatever the pen is set to.
			width = 10 * float32(s.scale)
		}
		return shots.Shape{Tool: s.tool, Color: col, Width: width, Rect: rect}, true
	case shots.ToolArrow:
		if !s.drawing || s.start == s.cur {
			return shots.Shape{}, false
		}
		return shots.Shape{Tool: shots.ToolArrow, Color: col, Width: width,
			From: s.start, To: s.cur}, true
	case shots.ToolPen:
		if !s.drawing || len(s.pen) == 0 {
			return shots.Shape{}, false
		}
		return shots.Shape{Tool: shots.ToolPen, Color: col, Width: width,
			Points: append([]image.Point(nil), s.pen...)}, true
	case shots.ToolText:
		if !s.typing || strings.TrimSpace(s.text) == "" {
			return shots.Shape{}, false
		}
		return shots.Shape{Tool: shots.ToolText, Color: col, At: s.textAt,
			Text: s.text, Size: 16 * float32(s.scale)}, true
	}
	return shots.Shape{}, false
}

// grab takes the topmost shape under pt, if any, to be dragged: what comes
// last is what the eye sees on top.
func (s *Screenshot) grab(pt image.Point) {
	s.sel = s.shapeAt(pt)
	if s.sel < 0 {
		s.moving = false
		return
	}
	s.moving, s.moved = true, false
	s.moveFrom, s.moveBase = pt, s.shapes[s.sel]
}

// shapeAt is the index of the topmost shape under pt, or -1.
func (s *Screenshot) shapeAt(pt image.Point) int {
	for i := len(s.shapes) - 1; i >= 0; i-- {
		if s.shapes[i].Hit(pt, 4) {
			return i
		}
	}
	return -1
}

// dragSelection moves the shape the pointer took with it, from where it lay
// as the drag began.
func (s *Screenshot) dragSelection(x, y float32) {
	if s.sel < 0 || s.sel >= len(s.shapes) || s.img == nil || s.k <= 0 {
		return
	}
	pt := s.clampImage(x, y)
	dx, dy := pt.X-s.moveFrom.X, pt.Y-s.moveFrom.Y
	if dx == 0 && dy == 0 {
		return
	}
	// The shape stays over the picture, however far the drag goes.
	b, frame := s.moveBase.Bounds(), s.img.Bounds()
	loX, hiX := frame.Min.X-b.Min.X, frame.Max.X-b.Max.X
	if loX > hiX {
		loX, hiX = hiX, loX
	}
	loY, hiY := frame.Min.Y-b.Min.Y, frame.Max.Y-b.Max.Y
	if loY > hiY {
		loY, hiY = hiY, loY
	}
	dx, dy = min(max(dx, loX), hiX), min(max(dy, loY), hiY)
	if !s.moved {
		// The state as the drag began, shape where it lay: undo goes back
		// to it.
		prev := append([]shots.Shape(nil), s.shapes...)
		prev[s.sel] = s.moveBase
		s.past = append(s.past, prev)
		s.future = nil
		s.moved = true
	}
	s.shapes[s.sel] = s.moveBase.Moved(dx, dy)
}

func (s *Screenshot) commit(sh shots.Shape) {
	s.keep()
	s.shapes = append(s.shapes, sh)
}

func (s *Screenshot) commitText() {
	if sh, ok := s.pendingShape(); ok {
		s.commit(sh)
	}
	s.typing, s.text = false, ""
}

// maxUndo is how many states the history keeps: enough to go back through a
// session of annotating, and no more.
const maxUndo = 100

// keep remembers the shapes as they are before a change. A change is undone
// by going back to a whole state, as a shape can be moved as well as added.
func (s *Screenshot) keep() {
	s.past = append(s.past, append([]shots.Shape(nil), s.shapes...))
	if len(s.past) > maxUndo {
		s.past = s.past[len(s.past)-maxUndo:]
	}
	s.future = nil
}

func (s *Screenshot) undo() {
	if len(s.past) == 0 {
		return
	}
	s.future = append(s.future, append([]shots.Shape(nil), s.shapes...))
	s.shapes = s.past[len(s.past)-1]
	s.past = s.past[:len(s.past)-1]
	s.afterHistory()
}

func (s *Screenshot) redoStep() {
	if len(s.future) == 0 {
		return
	}
	s.past = append(s.past, append([]shots.Shape(nil), s.shapes...))
	s.shapes = s.future[len(s.future)-1]
	s.future = s.future[:len(s.future)-1]
	s.afterHistory()
}

// afterHistory drops a selection that a state change left behind.
func (s *Screenshot) afterHistory() {
	if s.sel >= len(s.shapes) {
		s.sel = -1
	}
}

// removeSel deletes the shape the pointer took, as Delete does.
func (s *Screenshot) removeSel() {
	if s.sel < 0 || s.sel >= len(s.shapes) {
		return
	}
	s.keep()
	s.shapes = append(s.shapes[:s.sel], s.shapes[s.sel+1:]...)
	if s.sel >= len(s.shapes) {
		s.sel = -1
	}
}

func (s *Screenshot) clear() {
	s.img, s.bmp = nil, nil
	s.shapes, s.past, s.future = nil, nil, nil
	s.sel = -1
	s.drawing, s.typing, s.text = false, false, ""
	s.textAt = image.Point{}
	s.k = 0
}

func (s *Screenshot) copyImage(c *ui.Context) {
	if s.img == nil {
		return
	}
	data, err := shots.Encode(s.img, s.shapes)
	if err != nil {
		c.Toast("复制失败：" + err.Error())
		return
	}
	if err := shots.CopyPNG(data); err != nil {
		c.Toast("复制失败：" + err.Error())
		return
	}
	c.Toast("已复制到剪贴板")
}

func (s *Screenshot) saveImage(c *ui.Context) {
	if s.img == nil || s.win == nil {
		return
	}
	data, err := shots.Encode(s.img, s.shapes)
	if err != nil {
		c.Toast("保存失败：" + err.Error())
		return
	}
	win, said := s.win, s.said
	go func() {
		path, err := shots.SavePNG(win, data)
		switch {
		case err != nil:
			said <- "保存失败：" + err.Error()
		case path != "":
			said <- "已保存到 " + path
		}
	}()
	c.Services().Invalidate()
}

// imageAt maps a pointer position in the canvas to a pixel of the picture.
func (s *Screenshot) imageAt(x, y float32) (image.Point, bool) {
	if s.img == nil || s.k <= 0 {
		return image.Point{}, false
	}
	px := x / s.k
	py := y / s.k
	if px < 0 || py < 0 || px >= float32(s.img.Bounds().Dx()) || py >= float32(s.img.Bounds().Dy()) {
		return image.Point{}, false
	}
	return image.Pt(int(px), int(py)), true
}

// clampImage is imageAt for a drag that went past the picture: the edge is
// where it stops.
func (s *Screenshot) clampImage(x, y float32) image.Point {
	if pt, ok := s.imageAt(x, y); ok {
		return pt
	}
	if s.img == nil || s.k <= 0 {
		return image.Point{}
	}
	px := min(max(x/s.k, 0), float32(s.img.Bounds().Dx()-1))
	py := min(max(y/s.k, 0), float32(s.img.Bounds().Dy()-1))
	return image.Pt(int(px), int(py))
}
