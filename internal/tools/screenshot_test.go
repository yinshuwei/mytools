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
