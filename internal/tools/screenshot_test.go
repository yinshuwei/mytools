package tools

import (
	"image"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/yinshuwei/mytools/internal/shots"
)

// TestShotImageAt checks the way a pointer on the canvas becomes a pixel of
// the picture: the picture is fitted, so its pixels are not the window's.
func TestShotImageAt(t *testing.T) {
	s := &Screenshot{
		img: image.NewRGBA(image.Rect(0, 0, 100, 50)),
		k:   2,
	}
	if pt, ok := s.imageAt(100, 50); !ok || pt != image.Pt(50, 25) {
		t.Errorf("点 = %v，%v，want (50,25)", pt, ok)
	}
	if _, ok := s.imageAt(-1, 5); ok {
		t.Error("画面左边外也算点中")
	}
	if _, ok := s.imageAt(200, 100); ok {
		t.Error("画面右边外也算点中")
	}
	// A drag that leaves the picture stops at its edge.
	if got := s.clampImage(1000, 1000); got != image.Pt(99, 49) {
		t.Errorf("拖出画面 = %v，want (99,49)", got)
	}
	if got := s.clampImage(-100, -100); got != image.Pt(0, 0) {
		t.Errorf("拖出画面 = %v，want (0,0)", got)
	}
}

// TestShotPendingShape checks the shape a drag makes, in the picture's
// pixels: the stroke is in DIPs on screen, but saved at the picture's scale.
func TestShotPendingShape(t *testing.T) {
	s := &Screenshot{
		img:     image.NewRGBA(image.Rect(0, 0, 100, 100)),
		scale:   2,
		tool:    shots.ToolRect,
		color:   0,
		width:   3,
		drawing: true,
		start:   image.Pt(30, 40),
		cur:     image.Pt(10, 20),
	}
	sh, ok := s.pendingShape()
	if !ok {
		t.Fatal("没有生成矩形")
	}
	if sh.Rect != image.Rect(10, 20, 30, 40) {
		t.Errorf("矩形 = %v，want (10,20)-(30,40)", sh.Rect)
	}
	if sh.Width != 6 {
		t.Errorf("线宽 = %v，want 6（3 DIP × 2）", sh.Width)
	}

	// A pen keeps the path it went over.
	s.tool, s.pen = shots.ToolPen, []image.Point{{X: 1, Y: 1}, {X: 5, Y: 8}}
	if sh, ok := s.pendingShape(); !ok || len(sh.Points) != 2 {
		t.Errorf("画笔 = %+v，%v", sh, ok)
	}

	// Nothing is made of a click that did not drag.
	s.tool, s.cur = shots.ToolRect, s.start
	if _, ok := s.pendingShape(); ok {
		t.Error("没拖动却生成了矩形")
	}
}

// TestShotUndoRedo checks the history: undo goes back a state, redo forward
// again, and a new change drops what could be redone.
func TestShotUndoRedo(t *testing.T) {
	s := &Screenshot{}
	s.commit(shots.Shape{Tool: shots.ToolRect})
	s.commit(shots.Shape{Tool: shots.ToolArrow})
	s.undo()
	if len(s.shapes) != 1 || len(s.future) != 1 {
		t.Fatalf("撤销后 shapes=%d future=%d", len(s.shapes), len(s.future))
	}
	s.redoStep()
	if len(s.shapes) != 2 || len(s.future) != 0 {
		t.Errorf("重做后 shapes=%d future=%d", len(s.shapes), len(s.future))
	}
	s.undo()
	s.commit(shots.Shape{Tool: shots.ToolText})
	if len(s.future) != 0 {
		t.Error("新画一笔后还能重做")
	}
}

// TestShotMoveShape checks the select tool: a drag moves the shape under the
// pointer, out of the picture no further than its edge, and undo puts it
// back where it lay.
func TestShotMoveShape(t *testing.T) {
	s := &Screenshot{
		img:   image.NewRGBA(image.Rect(0, 0, 100, 100)),
		k:     1,
		scale: 1,
		tool:  shots.ToolSelect,
		sel:   -1,
		shapes: []shots.Shape{{
			Tool: shots.ToolRect, Width: 2, Rect: image.Rect(10, 10, 30, 30),
		}},
	}
	drag := func(x0, y0, x1, y1 float32) {
		s.input(ui.InputEvent{Kind: ui.InputPointerDown, X: x0, Y: y0})
		s.input(ui.InputEvent{Kind: ui.InputPointerMove, X: x1, Y: y1})
		s.input(ui.InputEvent{Kind: ui.InputPointerUp, X: x1, Y: y1})
	}
	drag(20, 20, 40, 50)
	if got := s.shapes[0].Rect; got != image.Rect(30, 40, 50, 60) {
		t.Fatalf("拖动后 = %v，want (30,40)-(50,60)", got)
	}
	s.undo()
	if got := s.shapes[0].Rect; got != image.Rect(10, 10, 30, 30) {
		t.Errorf("撤销后 = %v，want 原位", got)
	}

	// A drag that goes past the picture keeps the shape over it.
	s.redoStep()
	drag(40, 50, 500, 500)
	if got := s.shapes[0].Bounds(); got.Max.X != 100 || got.Max.Y != 100 {
		t.Errorf("拖出画面后 = %v，want 贴住右下角", got)
	}

	// A press on nothing takes the selection away, and leaves the history
	// alone.
	s.sel = 0
	before := len(s.past)
	s.input(ui.InputEvent{Kind: ui.InputPointerDown, X: 90, Y: 5})
	if s.sel != -1 {
		t.Error("点空白处没有取消选择")
	}
	if len(s.past) != before {
		t.Errorf("点空白处多了一步历史：%d", len(s.past))
	}
}

// TestShotTextFloats checks that the text field floats over the picture: the
// canvas keeps its box as the field comes, and Enter puts the text down.
func TestShotTextFloats(t *testing.T) {
	s := &Screenshot{}
	s.SetImage(image.NewRGBA(image.Rect(0, 0, 400, 300)), 1)
	tr := ui.NewTester(s.View, 900, 600)
	tr.Frame()
	before, ok := tr.Find("画布")
	if !ok {
		t.Fatal("没有找到画布")
	}
	if err := tr.Click("文字"); err != nil {
		t.Fatal(err)
	}
	tr.Frame()
	if err := tr.Click("画布"); err != nil {
		t.Fatal(err)
	}
	tr.Frame()
	if !s.typing {
		t.Fatal("点画布没有开始输入文字")
	}
	after, _ := tr.Find("画布")
	if after != before {
		t.Errorf("文字输入把画布挤动了：%v → %v", before, after)
	}
	// A press on the field is a press on the field: the text stays where it
	// was put, at the middle of the picture.
	at := s.textAt
	tr.ClickAt(after.X+float32(at.X)+110, after.Y+float32(at.Y)+22+16)
	tr.Frame()
	if s.textAt != at || !s.typing {
		t.Errorf("点浮层后 at=%v typing=%v，want %v、true", s.textAt, s.typing, at)
	}
	tr.Type("你好")
	tr.Key(0, ui.KeyEnter)
	tr.Frame()
	if len(s.shapes) != 1 || s.shapes[0].Text != "你好" {
		t.Fatalf("Enter 后 shapes = %+v", s.shapes)
	}
	if s.typing {
		t.Error("Enter 后还在输入")
	}
	if s.tool != shots.ToolSelect {
		t.Errorf("放下文字后工具 = %v，want 选择", shots.Names[s.tool])
	}
}

// TestShotLayersPanel checks the panel beside the picture: the shape drawn
// last at the top, the arrows that move a shape over and under the others,
// and the × that deletes it.
func TestShotLayersPanel(t *testing.T) {
	s := &Screenshot{}
	s.SetImage(image.NewRGBA(image.Rect(0, 0, 200, 150)), 1)
	s.shapes = []shots.Shape{
		{Tool: shots.ToolRect, Rect: image.Rect(0, 0, 20, 20)},
		{Tool: shots.ToolArrow, From: image.Pt(0, 0), To: image.Pt(30, 30)},
	}
	tr := ui.NewTester(s.View, 900, 600)
	tr.Frame()
	if !tr.HasText("箭头") || !tr.HasText("矩形") {
		t.Fatalf("面板里没有两行：%v", tr.Texts())
	}

	// The pointer takes a shape by its row; the rows go from the topmost,
	// which is the one drawn last.
	if err := tr.Click("图层 1：箭头"); err != nil {
		t.Fatal(err)
	}
	if s.sel != 1 {
		t.Errorf("点第一行后 sel = %d，want 1", s.sel)
	}

	// A press on a shape of the picture takes it, and the panel follows:
	// the row of the rectangle is the one now lit. The point is off the
	// arrow's line, which runs over the rectangle's corner.
	canvas, ok := tr.Find("画布")
	if !ok {
		t.Fatal("没有找到画布")
	}
	tr.ClickAt(canvas.X+5, canvas.Y+15)
	if s.sel != 0 {
		t.Errorf("点画布上的矩形后 sel = %d，want 0", s.sel)
	}

	// The arrow is on top: pressing its up leaves the order alone, while
	// the rectangle under it goes over it.
	tr.Click("第1层：往上挪一层，盖住上面那个")
	if s.shapes[0].Tool != shots.ToolRect || s.shapes[1].Tool != shots.ToolArrow {
		t.Fatalf("最上层被挪动了：%v, %v", s.shapes[0].Tool, s.shapes[1].Tool)
	}
	if err := tr.Click("第2层：往上挪一层，盖住上面那个"); err != nil {
		t.Fatal(err)
	}
	if s.shapes[0].Tool != shots.ToolArrow || s.shapes[1].Tool != shots.ToolRect {
		t.Fatalf("上移后 = %v, %v，want 箭头在下、矩形在上",
			s.shapes[0].Tool, s.shapes[1].Tool)
	}

	// Down puts it back under, and undo goes back to either state.
	if err := tr.Click("第1层：往下挪一层，被下面那个盖住"); err != nil {
		t.Fatal(err)
	}
	if s.shapes[0].Tool != shots.ToolRect {
		t.Fatal("下移没有回到原位")
	}
	s.undo()
	if s.shapes[0].Tool != shots.ToolArrow {
		t.Error("撤销没有回到上移后的状态")
	}

	// The × deletes the shape of its row, and undo brings it back.
	tr.Frame()
	if err := tr.Click("第1层：删掉这一层"); err != nil {
		t.Fatal(err)
	}
	if len(s.shapes) != 1 || s.shapes[0].Tool != shots.ToolArrow {
		t.Fatalf("删掉后 = %+v", s.shapes)
	}
	s.undo()
	if len(s.shapes) != 2 {
		t.Errorf("撤销删除后 = %d 个", len(s.shapes))
	}
}

// TestShotToolAfterCommit checks that a shape leaves the pointer in the
// select tool, on the shape just made: it is ready to be moved or deleted.
func TestShotToolAfterCommit(t *testing.T) {
	s := &Screenshot{
		img:   image.NewRGBA(image.Rect(0, 0, 100, 100)),
		k:     1,
		scale: 1,
		tool:  shots.ToolRect,
		color: 0,
		width: 3,
	}
	s.input(ui.InputEvent{Kind: ui.InputPointerDown, X: 10, Y: 10})
	s.input(ui.InputEvent{Kind: ui.InputPointerMove, X: 40, Y: 40})
	s.input(ui.InputEvent{Kind: ui.InputPointerUp, X: 40, Y: 40})
	if len(s.shapes) != 1 || s.shapes[0].Rect != image.Rect(10, 10, 40, 40) {
		t.Fatalf("画完 = %+v", s.shapes)
	}
	if s.tool != shots.ToolSelect {
		t.Errorf("画完的工具 = %v，want 选择", shots.Names[s.tool])
	}
	if s.sel != 0 {
		t.Errorf("画完选中 = %d，want 0", s.sel)
	}
}

// TestShotEllipseByDrag checks the circle tool on the page: a drag makes one,
// and the pointer is left in the select tool on it.
func TestShotEllipseByDrag(t *testing.T) {
	s := &Screenshot{}
	s.SetImage(image.NewRGBA(image.Rect(0, 0, 300, 200)), 1)
	tr := ui.NewTester(s.View, 900, 600)
	tr.Frame()
	if err := tr.Click("圆圈"); err != nil {
		t.Fatal(err)
	}
	tr.Frame()
	canvas, ok := tr.Find("画布")
	if !ok {
		t.Fatal("没有找到画布")
	}
	tr.Press(canvas.X+20, canvas.Y+20)
	tr.Move(canvas.X+120, canvas.Y+80)
	tr.Release(canvas.X+120, canvas.Y+80)
	tr.Frame()
	if len(s.shapes) != 1 || s.shapes[0].Tool != shots.ToolEllipse {
		t.Fatalf("拖出来的 = %+v", s.shapes)
	}
	if got := s.shapes[0].Rect; got != image.Rect(20, 20, 120, 80) {
		t.Errorf("圆圈框 = %v，want (20,20)-(120,80)", got)
	}
	if s.tool != shots.ToolSelect || s.sel != 0 {
		t.Errorf("画完 tool=%v sel=%d，want 选择、选中新圆圈", s.tool, s.sel)
	}
}

// TestShotNudge checks the arrow keys on the shape the pointer took: one
// pixel at a time, ten with Shift, and undo for each step.
func TestShotNudge(t *testing.T) {
	s := &Screenshot{}
	s.SetImage(image.NewRGBA(image.Rect(0, 0, 100, 100)), 1)
	s.shapes = []shots.Shape{{Tool: shots.ToolRect, Width: 2, Rect: image.Rect(10, 10, 30, 30)}}
	s.tool, s.sel = shots.ToolSelect, 0
	tr := ui.NewTester(s.View, 900, 600)
	tr.Frame()

	tr.Key(0, ui.KeyRight)
	tr.Frame()
	if got := s.shapes[0].Rect; got.Min.X != 11 {
		t.Errorf("→ 之后 = %v，want 左边 11", got)
	}
	tr.Key(ui.Shift, ui.KeyDown)
	tr.Frame()
	if got := s.shapes[0].Rect; got.Min.Y != 20 {
		t.Errorf("Shift+↓ 之后 = %v，want 上边 20", got)
	}
	s.undo()
	if got := s.shapes[0].Rect; got.Min.X != 11 || got.Min.Y != 10 {
		t.Errorf("撤销后 = %v，want (11,10)", got)
	}
}

// TestShotRecolor checks that a swatch paints the shape the pointer took, as
// it does the pen, and that a mosaic keeps the picture's colors.
func TestShotRecolor(t *testing.T) {
	s := &Screenshot{}
	s.SetImage(image.NewRGBA(image.Rect(0, 0, 100, 100)), 1)
	s.shapes = []shots.Shape{
		{Tool: shots.ToolRect, Color: shotColors[0].col, Width: 2, Rect: image.Rect(0, 0, 10, 10)},
		{Tool: shots.ToolMosaic, Width: 8, Rect: image.Rect(20, 20, 40, 40)},
	}
	s.sel = 0
	s.recolor(shotColors[4].col)
	if s.shapes[0].Color != shotColors[4].col {
		t.Errorf("改色后 = %v，want %v", s.shapes[0].Color, shotColors[4].col)
	}
	s.sel = 1
	s.recolor(shotColors[4].col)
	if s.shapes[1].Color == shotColors[4].col {
		t.Error("马赛克被改了颜色")
	}
	s.undo()
	if s.shapes[0].Color != shotColors[0].col {
		t.Errorf("撤销后 = %v，want 原来的颜色", s.shapes[0].Color)
	}
}

// TestShotTextSize checks the size buttons: the text the pointer took grows
// by two DIPs a press, undo takes it back, and the next text is made at the
// size shown.
func TestShotTextSize(t *testing.T) {
	s := &Screenshot{}
	s.SetImage(image.NewRGBA(image.Rect(0, 0, 200, 100)), 1)
	s.shapes = []shots.Shape{{Tool: shots.ToolText, Text: "字", At: image.Pt(10, 10), Size: 16}}
	s.tool, s.sel = shots.ToolSelect, 0
	if !s.textInHand() {
		t.Error("选中文字时没有字号控件")
	}
	s.bumpText(2)
	if s.shapes[0].Size != 18 {
		t.Errorf("字号 = %v，want 18", s.shapes[0].Size)
	}
	s.undo()
	if s.shapes[0].Size != 16 {
		t.Errorf("撤销后字号 = %v，want 16", s.shapes[0].Size)
	}

	// A text put down afterwards is that big.
	s.tool, s.typing, s.text, s.textAt = shots.ToolText, true, "a", image.Pt(5, 5)
	s.textSize = 30
	sh, ok := s.pendingShape()
	if !ok || sh.Size != 30 {
		t.Errorf("新文字 = %+v，%v，want 30", sh, ok)
	}
}

// TestShotLayersReveal checks the binding the other way round: a shape taken
// on the picture lights its row, and the panel scrolls to it even when it is
// far down the list.
func TestShotLayersReveal(t *testing.T) {
	s := &Screenshot{}
	s.SetImage(image.NewRGBA(image.Rect(0, 0, 400, 400)), 1)
	for i := 0; i < 30; i++ {
		s.shapes = append(s.shapes, shots.Shape{
			Tool: shots.ToolRect, Width: 2,
			Rect: image.Rect(5, i*10, 15, i*10+8),
		})
	}
	s.tool, s.sel = shots.ToolSelect, -1
	tr := ui.NewTester(s.View, 900, 600)
	tr.Frame()
	canvas, ok := tr.Find("画布")
	if !ok {
		t.Fatal("没有找到画布")
	}
	// The first drawn is the last row: the 29 made after it are over it.
	tr.ClickAt(canvas.X+5, canvas.Y+5)
	tr.Frame()
	if s.sel != 0 {
		t.Fatalf("点画布后 sel = %d，want 0", s.sel)
	}
	row, ok := tr.Find("图层 30：矩形")
	if !ok {
		t.Fatal("面板没有滚到被选中的那一行")
	}
	if row.Y+row.H > 600 {
		t.Errorf("第 30 行在窗口外：%v", row)
	}
}

// TestShotRemoveShape checks Delete: the shape the pointer took goes, and
// undo brings it back.
func TestShotRemoveShape(t *testing.T) {
	s := &Screenshot{sel: 0, shapes: []shots.Shape{
		{Tool: shots.ToolRect, Rect: image.Rect(0, 0, 5, 5)},
		{Tool: shots.ToolText, Text: "a", At: image.Pt(10, 10)},
	}}
	s.removeSel()
	if len(s.shapes) != 1 || s.shapes[0].Tool != shots.ToolText {
		t.Fatalf("删除后 = %+v", s.shapes)
	}
	s.undo()
	if len(s.shapes) != 2 {
		t.Errorf("撤销删除后 = %d 个", len(s.shapes))
	}
}
