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

	shapes []shots.Shape
	redo   []shots.Shape
	tool   shots.Tool
	color  int     // index into shotColors
	width  float64 // stroke, in DIPs

	drawing bool
	start   image.Point
	cur     image.Point
	pen     []image.Point

	textAt    image.Point
	text      string
	typing    bool
	focusText bool

	// view is where the picture shows, in the canvas's own DIPs, and k how
	// many DIPs one of its pixels takes.
	view ui.Rect
	k    float32

	// said reports what a save came to, from the goroutine that ran the
	// dialog.
	said chan string
}

func (*Screenshot) ID() string   { return "shot" }
func (*Screenshot) Name() string { return "截图" }
func (*Screenshot) Desc() string { return "框选屏幕，在页面里标注、复制或保存" }

// MaxWidth leaves the editor the room of a page of controls.
func (*Screenshot) MaxWidth() float32 { return PageWidth }

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
	s.shapes, s.redo = nil, nil
	s.drawing, s.typing, s.text = false, false, ""
	s.textAt = image.Point{}
	s.view, s.k = ui.Rect{}, 0
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
		s.toolbar(c, t)
		if s.typing {
			s.textRow(c)
		}
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
		} else {
			s.clear()
		}
	}
}

func (s *Screenshot) toolbar(c *ui.Context, t *ui.Theme) {
	ui.Row(c).Gap(8).Wrap().AlignItems(ui.Center).Children(func() {
		tool := int(s.tool)
		if ui.Segmented(c.Key("tool"), &tool, shots.Names...).Changed() {
			s.tool = shots.Tool(tool)
			s.typing = false
		}
		ui.Spacer(c)
		if ui.Button(c.Key("undo"), "撤销").Disabled(len(s.shapes) == 0).
			Tooltip("撤销上一步（" + widgets.ShortcutHint(0, "Z") + "）").Clicked() {
			s.undo()
		}
		if ui.Button(c.Key("redo"), "重做").Disabled(len(s.redo) == 0).
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

// textRow is the field a text annotation is typed in, under the toolbar.
func (s *Screenshot) textRow(c *ui.Context) {
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		widgets.Caption(c, "文字")
		box := ui.TextInput(c.Key("text"), &s.text).
			Placeholder("输入要标注的文字，Enter 放下").
			Tooltip("Enter 把文字放在刚才点的地方，Esc 取消").Grow(1)
		if s.focusText {
			s.focusText = false
			box.Focus()
		}
		if box.Submitted() {
			s.commitText()
		}
		if ui.Button(c.Key("textok"), "放下").Clicked() {
			s.commitText()
		}
		if ui.Button(c.Key("textcancel"), "取消").Clicked() {
			s.typing, s.text = false, ""
		}
	})
}

func (s *Screenshot) canvasView(c *ui.Context) {
	cursor := ui.CursorCrosshair
	if s.tool == shots.ToolText {
		cursor = ui.CursorText
	}
	ui.Box(c.Key("canvas")).FillWidth().Grow(1).MinHeight(widgets.Scaled(140)).
		Label("画布").Cursor(cursor).HandleInput(s.input).Draw(s.draw)
}

func (s *Screenshot) status(c *ui.Context) {
	ui.Row(c).Gap(12).Wrap().AlignItems(ui.Center).Children(func() {
		widgets.Hint(c, fmt.Sprintf("%d × %d 像素", s.img.Bounds().Dx(), s.img.Bounds().Dy()))
		widgets.Hint(c, "拖动绘制 · "+widgets.ShortcutHint(0, "Z")+" 撤销 · "+
			widgets.ShortcutHint(0, "C")+" 复制 · "+widgets.ShortcutHint(0, "S")+" 保存")
	})
}

// draw paints the picture fitted into the canvas, the annotations over it,
// and the shape being drawn.
func (s *Screenshot) draw(p *ui.Painter, r ui.Rect) {
	p.Fill(r, ui.RGBA(0, 0, 0, 0.22), 0)
	if s.img == nil {
		return
	}
	const margin = 6
	area := ui.Rect{
		X: r.X + margin, Y: r.Y + margin,
		W: max(r.W-2*margin, 1), H: max(r.H-2*margin, 1),
	}
	view, k := shots.Fit(area, s.img.Bounds().Dx(), s.img.Bounds().Dy())
	if k <= 0 {
		return
	}
	s.view = ui.Rect{X: view.X - r.X, Y: view.Y - r.Y, W: view.W, H: view.H}
	s.k = k
	p.Image(s.bmp, view, ui.Contain)
	shots.Draw(p, s.shapes, view.X, view.Y, k, s.img)
	if sh, ok := s.pendingShape(); ok {
		shots.Draw(p, []shots.Shape{sh}, view.X, view.Y, k, s.img)
	}
	p.Stroke(view, ui.RGBA(255, 255, 255, 0.35), 0, 1)
}

// input draws with the pointer: a drag makes a shape, a click of the text
// tool says where the text goes.
func (s *Screenshot) input(ev ui.InputEvent) bool {
	switch ev.Kind {
	case ui.InputPointerDown:
		if ev.Button != 0 {
			return false
		}
		pt, ok := s.imageAt(ev.X, ev.Y)
		if !ok {
			return false
		}
		if s.tool == shots.ToolText {
			s.textAt, s.text, s.typing, s.focusText = pt, "", true, true
			return true
		}
		s.drawing = true
		s.start, s.cur = pt, pt
		s.pen = []image.Point{pt}
		return true
	case ui.InputPointerMove:
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
		if ev.Button != 0 || !s.drawing {
			return false
		}
		s.drawing = false
		s.cur = s.clampImage(ev.X, ev.Y)
		if s.tool == shots.ToolPen {
			s.pen = append(s.pen, s.cur)
		}
		if sh, ok := s.pendingShape(); ok {
			s.commit(sh)
		}
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

func (s *Screenshot) commit(sh shots.Shape) {
	s.shapes = append(s.shapes, sh)
	s.redo = nil
}

func (s *Screenshot) commitText() {
	if sh, ok := s.pendingShape(); ok {
		s.commit(sh)
	}
	s.typing, s.text = false, ""
}

func (s *Screenshot) undo() {
	if len(s.shapes) == 0 {
		return
	}
	last := s.shapes[len(s.shapes)-1]
	s.shapes = s.shapes[:len(s.shapes)-1]
	s.redo = append(s.redo, last)
}

func (s *Screenshot) redoStep() {
	if len(s.redo) == 0 {
		return
	}
	last := s.redo[len(s.redo)-1]
	s.redo = s.redo[:len(s.redo)-1]
	s.shapes = append(s.shapes, last)
}

func (s *Screenshot) clear() {
	s.img, s.bmp = nil, nil
	s.shapes, s.redo = nil, nil
	s.drawing, s.typing, s.text = false, false, ""
	s.textAt = image.Point{}
	s.view, s.k = ui.Rect{}, 0
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
	px := (x - s.view.X) / s.k
	py := (y - s.view.Y) / s.k
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
	px := min(max((x-s.view.X)/s.k, 0), float32(s.img.Bounds().Dx()-1))
	py := min(max((y-s.view.Y)/s.k, 0), float32(s.img.Bounds().Dy()-1))
	return image.Pt(int(px), int(py))
}
