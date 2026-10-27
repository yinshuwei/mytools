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

// defaultTextSize is how big a text is put down, in DIPs, until the page is
// told otherwise.
const defaultTextSize = 16

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

	tool     shots.Tool
	color    int     // index into shotColors
	width    float64 // stroke, in DIPs
	textSize float64 // how big a text is, in DIPs

	drawing bool
	start   image.Point
	cur     image.Point
	pen     []image.Point

	// The shape the pointer took, and where it was taken from: dragging it
	// moves it, and dragging one of its grips sizes it. reveal asks the
	// layers panel to scroll the row of a shape taken on the picture into
	// view.
	sel      int
	moving   bool
	moved    bool
	reveal   bool
	grip     int // the grip in hand, or -1 for the shape itself
	moveFrom image.Point
	base     shots.Shape // the shape as the drag began

	textAt    image.Point
	text      string
	typing    bool
	focusText bool
	// A text opened by a double click is edited where it lies: editing holds
	// it and its shape as it was, so that Escape can put the words back.
	editing  bool
	editAt   int
	editBase shots.Shape
	// widthing is a drag of the width slider in hand: the whole drag makes
	// one step of history, not one for every pixel it passes.
	widthing bool

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
	if s.textSize <= 0 {
		s.textSize = defaultTextSize
	}
	s.shapes, s.past, s.future = nil, nil, nil
	s.sel = -1
	s.editing, s.editAt = false, 0
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
	// A text opened by a double click shows what is typed as it comes, so
	// that the words are seen in place, not in the field.
	if s.editing && s.editAt >= 0 && s.editAt < len(s.shapes) &&
		s.shapes[s.editAt].Text != s.text {
		s.shapes[s.editAt].Text = s.text
	}
	s.shortcuts(c)
	ui.Column(c).Grow(1).FillWidth().Gap(10).Children(func() {
		// The controls stay in a column a hand can reach, as on the other
		// pages; the picture below them takes the whole width.
		ui.Column(c).FillWidth().MaxWidth(PageWidth).Gap(10).Children(func() {
			s.toolbar(c, t)
		})
		ui.Row(c).FillWidth().Grow(1).Gap(10).AlignItems(ui.Stretch).Children(func() {
			s.canvasView(c)
			s.layers(c, t)
		})
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
		ui.Text(c, "图片会回到这里，可以画矩形、圆圈、箭头、画笔、文字和马赛克，再复制或保存。").
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
			s.cancelText()
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
	if s.sel >= 0 && (c.Shortcut(0, ui.KeyDelete) || c.Shortcut(0, ui.KeyBackspace)) {
		s.removeSel()
	}
	// The arrows step the shape the pointer took: a pixel at a time, or ten
	// with Shift held, as a picture is nudged pixel by pixel.
	if s.sel >= 0 {
		switch {
		case c.Shortcut(0, ui.KeyLeft):
			s.nudge(-1, 0)
		case c.Shortcut(ui.Shift, ui.KeyLeft):
			s.nudge(-10, 0)
		case c.Shortcut(0, ui.KeyRight):
			s.nudge(1, 0)
		case c.Shortcut(ui.Shift, ui.KeyRight):
			s.nudge(10, 0)
		case c.Shortcut(0, ui.KeyUp):
			s.nudge(0, -1)
		case c.Shortcut(ui.Shift, ui.KeyUp):
			s.nudge(0, -10)
		case c.Shortcut(0, ui.KeyDown):
			s.nudge(0, 1)
		case c.Shortcut(ui.Shift, ui.KeyDown):
			s.nudge(0, 10)
		}
	}
}

func (s *Screenshot) toolbar(c *ui.Context, t *ui.Theme) {
	ui.Row(c).Gap(8).Wrap().AlignItems(ui.Center).Children(func() {
		tool := int(s.tool)
		if ui.Segmented(c.Key("tool"), &tool, shots.Names...).Changed() {
			s.tool = shots.Tool(tool)
			s.cancelText()
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
				s.recolor(shotColors[i].col)
			}
		}
		widgets.Hint(c, "线宽")
		stroke := ui.Slider(c.Key("width"), &s.width, 1, 10).Width(widgets.Scaled(90)).
			Label("线宽滑块").Tooltip("画出来有多粗，以屏幕上的 DIP 计；选中标注时改的就是它")
		if stroke.Changed() {
			s.changeWidth()
		} else {
			s.widthing = false
		}
		if s.textInHand() {
			s.textSizeControl(c, t)
		}
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

// textInHand says whether the text size is worth showing: a text is being
// typed, or one is taken.
func (s *Screenshot) textInHand() bool {
	if s.tool == shots.ToolText {
		return true
	}
	return s.sel >= 0 && s.sel < len(s.shapes) && s.shapes[s.sel].Tool == shots.ToolText
}

// textSizeControl is the pair of buttons that make a text bigger or smaller:
// the one taken, and the ones made from now on.
func (s *Screenshot) textSizeControl(c *ui.Context, t *ui.Theme) {
	widgets.Hint(c, "字号")
	if layerButton(c, t, "sizesmall", "−", "字号小一号", true) {
		s.bumpText(-2)
	}
	ui.Text(c, strconv.Itoa(int(s.textDIP()))).FontSize(widgets.Scaled(12)).
		TextColor(t.Text)
	if layerButton(c, t, "sizebig", "+", "字号大一号", true) {
		s.bumpText(2)
	}
}

// bumpText makes a text bigger or smaller by d DIPs: the one taken if it is
// a text, and the ones made from now on.
func (s *Screenshot) bumpText(d int) {
	size := min(max(s.textDIP()+float64(d), 8), 96)
	s.textSize = size
	if s.sel >= 0 && s.sel < len(s.shapes) && s.shapes[s.sel].Tool == shots.ToolText {
		s.keep()
		s.shapes[s.sel].Size = float32(size) * float32(s.scale)
	}
}

// textDIP is how big a text is, in DIPs.
func (s *Screenshot) textDIP() float64 {
	if s.textSize <= 0 {
		return defaultTextSize
	}
	return s.textSize
}

// recolor paints the shape the pointer took in col, as the swatches do: a
// mosaic has no pen of its own.
func (s *Screenshot) recolor(col color.RGBA) {
	if s.sel < 0 || s.sel >= len(s.shapes) {
		return
	}
	sh := s.shapes[s.sel]
	if sh.Tool == shots.ToolMosaic || sh.Color == col {
		return
	}
	s.keep()
	s.shapes[s.sel].Color = col
}

// took points the pointer at the shape at i, and the toolbar at what it
// looks like: the pen and the stroke it was drawn with, and the size of its
// text.
func (s *Screenshot) took(i int) {
	s.sel = i
	if i < 0 || i >= len(s.shapes) || s.scale <= 0 {
		return
	}
	sh := s.shapes[i]
	for j, pen := range shotColors {
		if pen.col == sh.Color {
			s.color = j
			break
		}
	}
	if sh.Tool != shots.ToolText && sh.Tool != shots.ToolMosaic {
		s.width = min(max(float64(sh.Width)/s.scale, 1), 10)
	}
	if sh.Tool == shots.ToolText && s.scale > 0 {
		s.textSize = float64(sh.Size) / s.scale
	}
}

// nudge steps the shape the pointer took by a few pixels, as the arrow keys
// do.
func (s *Screenshot) nudge(dx, dy int) {
	if s.sel < 0 || s.sel >= len(s.shapes) || s.img == nil {
		return
	}
	sh := s.shapes[s.sel]
	dx, dy = clampMove(sh, s.img.Bounds(), dx, dy)
	if dx == 0 && dy == 0 {
		return
	}
	s.keep()
	s.shapes[s.sel] = sh.Moved(dx, dy)
}

// clampMove keeps a shape that moves by dx, dy over the picture.
func clampMove(sh shots.Shape, frame image.Rectangle, dx, dy int) (int, int) {
	b := sh.Bounds()
	loX, hiX := frame.Min.X-b.Min.X, frame.Max.X-b.Max.X
	if loX > hiX {
		loX, hiX = hiX, loX
	}
	loY, hiY := frame.Min.Y-b.Min.Y, frame.Max.Y-b.Max.Y
	if loY > hiY {
		loY, hiY = hiY, loY
	}
	return min(max(dx, loX), hiX), min(max(dy, loY), hiY)
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
	ui.ScrollBoth(c.Key("scroll")).Grow(1).Shrink(1).
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

// layers is the panel beside the picture: one row per annotation, the one
// drawn last at the top, to take it, to move it over the others or under
// them, and to delete it.
func (s *Screenshot) layers(c *ui.Context, t *ui.Theme) {
	ui.Column(c).Width(widgets.Scaled(190)).Shrink(0).Gap(6).
		Background(t.Surface).Border(1, t.Border).Radius(widgets.Scaled(8)).
		Padding(widgets.Scaled(8)).
		Children(func() {
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				widgets.Caption(c, "图层")
				ui.Spacer(c)
				widgets.Hint(c, strconv.Itoa(len(s.shapes))+" 层")
			})
			if len(s.shapes) == 0 {
				widgets.Hint(c, "还没有标注")
				return
			}
			// What a press asks for is taken once the rows are all built:
			// a shape that goes as the others are drawn would leave them
			// naming the wrong rows.
			act := layerAct{i: -1}
			ui.Scroll(c.Key("layers")).Grow(1).Children(func() {
				for i := len(s.shapes) - 1; i >= 0; i-- {
					s.layerRow(c, t, i, &act)
				}
			})
			act.apply(s)
		})
}

// layerAct is what a press on a row asks for: "up", "down" or "delete".
type layerAct struct {
	op string
	i  int
}

func (a layerAct) apply(s *Screenshot) {
	switch a.op {
	case "up":
		s.moveLayer(a.i, 1)
	case "down":
		s.moveLayer(a.i, -1)
	case "delete":
		s.removeShape(a.i)
	}
}

// layerRow is one annotation in the panel: its color and name, and the
// buttons that move it and delete it. A press on the row takes the shape.
func (s *Screenshot) layerRow(c *ui.Context, t *ui.Theme, i int, act *layerAct) {
	sh := s.shapes[i]
	// Rows are numbered as they are seen, from the top down, though the
	// shapes are kept the other way round.
	n := len(s.shapes) - i
	row := ui.Box(c.Key("layer"+strconv.Itoa(i))).FillWidth().Padding(3, 6).
		Radius(widgets.Scaled(4)).Cursor(ui.CursorPointer).
		Label("图层 " + strconv.Itoa(n) + "：" + layerName(sh))
	switch {
	case i == s.sel:
		row.Background(t.SurfaceHover).Border(1, t.Accent)
	case row.Hovered():
		row.Background(t.SurfaceHover)
	}
	row.Children(func() {
		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
			dot := ui.RGB(sh.Color.R, sh.Color.G, sh.Color.B)
			if sh.Tool == shots.ToolMosaic {
				// A mosaic has no pen of its own: it takes the picture's
				// colors.
				dot = t.Border
			}
			ui.Box(c).Size(widgets.Scaled(10), widgets.Scaled(10)).Shrink(0).
				Radius(widgets.Scaled(2)).Background(dot)
			ui.Text(c, layerName(sh)).FontSize(widgets.Scaled(12)).
				TextColor(t.Text).SingleLine().Grow(1)
			at := "第" + strconv.Itoa(n) + "层："
			if layerButton(c, t, "up"+strconv.Itoa(i), "↑", at+"往上挪一层，盖住上面那个",
				i < len(s.shapes)-1) {
				act.op, act.i = "up", i
			}
			if layerButton(c, t, "down"+strconv.Itoa(i), "↓", at+"往下挪一层，被下面那个盖住",
				i > 0) {
				act.op, act.i = "down", i
			}
			if layerButton(c, t, "del"+strconv.Itoa(i), "×", at+"删掉这一层", true) {
				act.op, act.i = "delete", i
			}
		})
	})
	if row.Clicked() {
		s.took(i)
		s.reveal = false
	}
	if i == s.sel && s.reveal {
		// A shape taken on the picture brings its row into view, once: the
		// panel is left where a hand scrolls it.
		s.reveal = false
		row.ScrollIntoView()
	}
}

// layerButton is one of the small buttons on a row: up, down, or the × that
// deletes the shape.
func layerButton(c *ui.Context, t *ui.Theme, key, glyph, tip string, on bool) bool {
	b := ui.ButtonBase(c.Key(key)).Size(widgets.Scaled(18), widgets.Scaled(18)).
		Radius(widgets.Scaled(3)).Shrink(0).Label(tip).Tooltip(tip)
	if !on {
		b.Disabled(true)
	} else if b.Hovered() {
		b.Background(t.SurfaceHover)
	}
	b.Children(func() {
		col := t.TextMuted
		if on {
			col = t.Text
		}
		ui.Text(c, glyph).FontSize(widgets.Scaled(11)).TextColor(col)
	})
	return on && b.Clicked()
}

// layerName is what a row says about a shape: its tool, and the words of a
// text.
func layerName(sh shots.Shape) string {
	name := "标注"
	if int(sh.Tool) < len(shots.Names) {
		name = shots.Names[sh.Tool]
	}
	if sh.Tool == shots.ToolText {
		if words := strings.TrimSpace(sh.Text); words != "" {
			return name + " · " + words
		}
	}
	return name
}

// moveLayer swaps a shape with the one beside it: by +1 is over the shape
// drawn after it, which is over it on the picture.
func (s *Screenshot) moveLayer(i, by int) {
	j := i + by
	if i < 0 || i >= len(s.shapes) || j < 0 || j >= len(s.shapes) {
		return
	}
	s.keep()
	s.shapes[i], s.shapes[j] = s.shapes[j], s.shapes[i]
	s.sel = j
}

// removeShape deletes the shape at i, and keeps the pointer on what it took.
func (s *Screenshot) removeShape(i int) {
	if i < 0 || i >= len(s.shapes) {
		return
	}
	s.keep()
	s.shapes = append(s.shapes[:i], s.shapes[i+1:]...)
	switch {
	case s.sel == i:
		s.sel = -1
	case s.sel > i:
		s.sel--
	}
}

func (s *Screenshot) status(c *ui.Context) {
	// Escape clears the picture earlier in the same frame, before this row
	// is built: there is no size to say until the landing page comes back.
	if s.img == nil {
		return
	}
	b := s.img.Bounds()
	ui.Row(c).Gap(12).Wrap().AlignItems(ui.Center).Children(func() {
		widgets.Hint(c, fmt.Sprintf("%d × %d 像素", b.Dx(), b.Dy()))
		widgets.Hint(c, "拖动绘制 · 选中后拖动可移动、拖小圆点缩放（箭头的端点调方向）· "+
			"方向键微调，按住 Shift 一次 10 像素 · Delete 删除")
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
	// hold on to; what is saved has no ring. An arrow goes without one: its
	// two ends say it is taken.
	if s.sel >= 0 && s.sel < len(s.shapes) {
		if s.shapes[s.sel].Tool != shots.ToolArrow {
			b := s.shapes[s.sel].Bounds()
			ring := ui.Rect{
				X: r.X + float32(b.Min.X)*s.k, Y: r.Y + float32(b.Min.Y)*s.k,
				W: float32(b.Dx()) * s.k, H: float32(b.Dy()) * s.k,
			}
			p.Stroke(ring, ui.RGBA(0, 0, 0, 0.45), 0, 3)
			p.Stroke(ring, ui.RGBA(255, 255, 255, 0.9), 0, 1)
		}
		s.drawGrips(p, r)
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
		if s.typing {
			// A press away from the field puts the words down as they are.
			s.commitText()
		}
		pt, ok := s.imageAt(ev.X, ev.Y)
		if !ok {
			s.sel = -1
			return false
		}
		switch s.tool {
		case shots.ToolSelect:
			// A grip is taken before the shape: the handles sit on it.
			if i := s.gripAt(ev.X, ev.Y); i >= 0 {
				s.startGrip(i)
				return true
			}
			// A double click on a text opens its words in the field.
			if ev.Clicks >= 2 {
				if i := s.shapeAt(pt); i >= 0 && s.shapes[i].Tool == shots.ToolText {
					s.editText(i)
					return true
				}
			}
			s.grab(pt)
			return true
		case shots.ToolText:
			s.textAt, s.text, s.typing, s.focusText = pt, "", true, true
			return true
		}
		// Drawing over the picture puts the shape the panel took aside.
		s.sel = -1
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
			s.moving, s.grip = false, -1
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
	case shots.ToolRect, shots.ToolEllipse, shots.ToolMosaic:
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
			Text: s.text, Size: float32(s.textDIP()) * float32(s.scale)}, true
	}
	return shots.Shape{}, false
}

// grab takes the topmost shape under pt, if any, to be dragged: what comes
// last is what the eye sees on top. The panel follows it to the same shape.
func (s *Screenshot) grab(pt image.Point) {
	s.took(s.shapeAt(pt))
	if s.sel < 0 {
		s.moving = false
		return
	}
	s.reveal = true
	s.moving, s.moved, s.grip = true, false, -1
	s.moveFrom, s.base = pt, s.shapes[s.sel]
}

// startGrip takes a grip of the shape the pointer took, to size it.
func (s *Screenshot) startGrip(i int) {
	s.grip = i
	s.moving, s.moved = true, false
	s.base = s.shapes[s.sel]
}

// gripRadius is how big a grip is drawn, in DIPs.
const gripRadius = 3.5

// gripAt is the grip of the shape the pointer took that (x, y), in the
// canvas's DIPs, is on, or -1.
func (s *Screenshot) gripAt(x, y float32) int {
	if s.sel < 0 || s.sel >= len(s.shapes) {
		return -1
	}
	reach := float32(gripRadius + 3.5)
	for i, g := range s.grips(s.shapes[s.sel]) {
		if dx, dy := x-g.x, y-g.y; dx*dx+dy*dy <= reach*reach {
			return i
		}
	}
	return -1
}

// grip is a point of a shape that sizes it, in DIPs inside the canvas.
type grip struct{ x, y float32 }

// grips are the points a shape is sized by: the two ends of an arrow, or the
// corners and the middles of the sides of a box. A text has none, as its size
// is its font's.
func (s *Screenshot) grips(sh shots.Shape) []grip {
	switch sh.Tool {
	case shots.ToolArrow:
		return []grip{
			{float32(sh.From.X) * s.k, float32(sh.From.Y) * s.k},
			{float32(sh.To.X) * s.k, float32(sh.To.Y) * s.k},
		}
	case shots.ToolRect, shots.ToolEllipse, shots.ToolMosaic:
		return boxGrips(sh.Rect, s.k)
	case shots.ToolPen:
		return boxGrips(sh.Bounds(), s.k)
	}
	return nil
}

// boxGrips are the eight points around a box, from the top-left corner round
// clockwise.
func boxGrips(r image.Rectangle, k float32) []grip {
	x0, y0 := float32(r.Min.X)*k, float32(r.Min.Y)*k
	x1, y1 := float32(r.Max.X)*k, float32(r.Max.Y)*k
	mx, my := (x0+x1)/2, (y0+y1)/2
	return []grip{
		{x0, y0}, {mx, y0}, {x1, y0}, {x1, my},
		{x1, y1}, {mx, y1}, {x0, y1}, {x0, my},
	}
}

// drawGrips paints the handles of the shape the pointer took: white dots with
// a dark ring, which stand out on any picture and are no part of what is
// saved.
func (s *Screenshot) drawGrips(p *ui.Painter, r ui.Rect) {
	if s.sel < 0 || s.sel >= len(s.shapes) {
		return
	}
	for _, g := range s.grips(s.shapes[s.sel]) {
		box := ui.Rect{
			X: r.X + g.x - gripRadius, Y: r.Y + g.y - gripRadius,
			W: 2 * gripRadius, H: 2 * gripRadius,
		}
		p.Fill(box, ui.RGBA(255, 255, 255, 0.95), gripRadius)
		p.Stroke(box, ui.RGBA(0, 0, 0, 0.55), gripRadius, 1)
	}
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
// as the drag began, or sizes it by the grip in hand.
func (s *Screenshot) dragSelection(x, y float32) {
	if s.sel < 0 || s.sel >= len(s.shapes) || s.img == nil || s.k <= 0 {
		return
	}
	pt := s.clampImage(x, y)
	if s.grip >= 0 {
		s.dragGrip(pt)
		return
	}
	dx, dy := pt.X-s.moveFrom.X, pt.Y-s.moveFrom.Y
	if dx == 0 && dy == 0 {
		return
	}
	// The shape stays over the picture, however far the drag goes.
	dx, dy = clampMove(s.base, s.img.Bounds(), dx, dy)
	s.pushDrag()
	s.shapes[s.sel] = s.base.Moved(dx, dy)
}

// pushDrag remembers the state as the drag began, the first time the drag
// changes anything: undo goes back to it.
func (s *Screenshot) pushDrag() {
	if s.moved {
		return
	}
	prev := append([]shots.Shape(nil), s.shapes...)
	prev[s.sel] = s.base
	s.past = append(s.past, prev)
	s.future = nil
	s.moved = true
}

// dragGrip sizes the shape by the grip in hand: a box takes the pointer as a
// corner or an edge, a pen's path is stretched to the box the grip makes, and
// an arrow's end follows the pointer, which turns it.
func (s *Screenshot) dragGrip(pt image.Point) {
	sh, next := s.base, s.base
	switch sh.Tool {
	case shots.ToolArrow:
		if s.grip == 0 {
			next.From = pt
		} else {
			next.To = pt
		}
		if next.From == sh.From && next.To == sh.To {
			return
		}
	case shots.ToolRect, shots.ToolEllipse, shots.ToolMosaic:
		next.Rect = resizeRect(sh.Rect, s.grip, pt)
		if next.Rect == sh.Rect {
			return
		}
	case shots.ToolPen:
		r := resizeRect(sh.Bounds(), s.grip, pt)
		if r == sh.Bounds() {
			return
		}
		next = sh.Fit(r)
	default:
		return
	}
	s.pushDrag()
	s.shapes[s.sel] = next
}

// resizeRect is the box a grip makes: the side opposite the grip stays where
// it was, and a drag past it turns the box over.
func resizeRect(r image.Rectangle, grip int, pt image.Point) image.Rectangle {
	x0, y0, x1, y1 := r.Min.X, r.Min.Y, r.Max.X, r.Max.Y
	switch grip {
	case 0:
		x0, y0 = pt.X, pt.Y
	case 1:
		y0 = pt.Y
	case 2:
		x1, y0 = pt.X, pt.Y
	case 3:
		x1 = pt.X
	case 4:
		x1, y1 = pt.X, pt.Y
	case 5:
		y1 = pt.Y
	case 6:
		x0, y1 = pt.X, pt.Y
	case 7:
		x0 = pt.X
	}
	if x1 < x0 {
		x0, x1 = x1, x0
	}
	if y1 < y0 {
		y0, y1 = y1, y0
	}
	return image.Rect(x0, y0, x1, y1)
}

// commit adds the shape that was just made and takes the pointer to it: what
// comes next is moving it, deleting it, or drawing another, and the tool for
// the first two is the one that takes it.
func (s *Screenshot) commit(sh shots.Shape) {
	s.keep()
	s.shapes = append(s.shapes, sh)
	s.tool = shots.ToolSelect
	s.sel = len(s.shapes) - 1
}

func (s *Screenshot) commitText() {
	if s.editing {
		s.finishEdit(true)
		return
	}
	if sh, ok := s.pendingShape(); ok {
		s.commit(sh)
	}
	s.typing, s.text = false, ""
}

// cancelText drops what is being typed: the words of a text being edited
// come back as they were.
func (s *Screenshot) cancelText() {
	if s.editing {
		s.finishEdit(false)
		return
	}
	s.typing, s.text = false, ""
}

// editText opens the field on the text at i, to change its words where they
// lie, with the picture showing them as they are typed.
func (s *Screenshot) editText(i int) {
	s.took(i)
	s.editing, s.editAt, s.editBase = true, i, s.shapes[i]
	s.text, s.textAt = s.shapes[i].Text, s.shapes[i].At
	s.typing, s.focusText, s.moved = true, true, false
}

// finishEdit closes the field on the text that was opened: what was typed
// takes the place of the words, in one step of history, or the words come
// back as they were.
func (s *Screenshot) finishEdit(keep bool) {
	i, base, words := s.editAt, s.editBase, s.text
	s.editing, s.typing, s.text = false, false, ""
	if i < 0 || i >= len(s.shapes) {
		return
	}
	if !keep || strings.TrimSpace(words) == "" || words == base.Text {
		s.shapes[i] = base
		return
	}
	s.shapes[i] = base
	s.shapes[i].Text = words
	prev := append([]shots.Shape(nil), s.shapes...)
	prev[i] = base
	s.past = append(s.past, prev)
	if len(s.past) > maxUndo {
		s.past = s.past[len(s.past)-maxUndo:]
	}
	s.future = nil
}

// changeWidth puts the width the slider holds on the shape the pointer took;
// a whole drag of the slider makes one step of history, not one a pixel.
func (s *Screenshot) changeWidth() {
	if s.sel < 0 || s.sel >= len(s.shapes) {
		return
	}
	sh := &s.shapes[s.sel]
	if sh.Tool == shots.ToolText || sh.Tool == shots.ToolMosaic {
		return
	}
	if !s.widthing {
		s.widthing = true
		s.keep()
	}
	sh.Width = float32(s.width) * float32(s.scale)
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
	if s.sel >= 0 {
		s.removeShape(s.sel)
	}
}

func (s *Screenshot) clear() {
	s.img, s.bmp = nil, nil
	s.shapes, s.past, s.future = nil, nil, nil
	s.sel = -1
	s.editing, s.editAt = false, 0
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
