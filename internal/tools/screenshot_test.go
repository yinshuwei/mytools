package tools

import (
	"image"
	"testing"

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

// TestShotUndoRedo checks the history: undo puts a shape aside, redo takes
// it back, and a new one drops what could be redone.
func TestShotUndoRedo(t *testing.T) {
	s := &Screenshot{shapes: []shots.Shape{{Tool: shots.ToolRect}, {Tool: shots.ToolArrow}}}
	s.undo()
	if len(s.shapes) != 1 || len(s.redo) != 1 {
		t.Fatalf("撤销后 shapes=%d redo=%d", len(s.shapes), len(s.redo))
	}
	s.redoStep()
	if len(s.shapes) != 2 || len(s.redo) != 0 {
		t.Errorf("重做后 shapes=%d redo=%d", len(s.shapes), len(s.redo))
	}
	s.undo()
	s.commit(shots.Shape{Tool: shots.ToolText})
	if len(s.redo) != 0 {
		t.Error("新画一笔后还能重做")
	}
}
