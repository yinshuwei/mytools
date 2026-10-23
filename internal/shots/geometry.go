package shots

import (
	"image"
	"math"
	"os"
	"path/filepath"
	"time"

	"github.com/egoist/mygo/ui"
)

// minSelection is the smallest drag, in DIPs, that makes a screenshot: a
// press with a pixel of jitter is not one.
const minSelection float32 = 6

// accent is the color of the selection and of the copy button.
var accent = ui.RGB(0, 122, 255)

// rectFromDrag is the rectangle between two corners of a drag, however it
// was dragged.
func rectFromDrag(x0, y0, x1, y1 float32) ui.Rect {
	return ui.Rect{
		X: min(x0, x1),
		Y: min(y0, y1),
		W: abs(x1 - x0),
		H: abs(y1 - y0),
	}
}

// clampSelection keeps r inside a w×h area.
func clampSelection(r ui.Rect, w, h float32) ui.Rect {
	x0 := clamp32(r.X, 0, w)
	y0 := clamp32(r.Y, 0, h)
	x1 := clamp32(r.X+r.W, 0, w)
	y1 := clamp32(r.Y+r.H, 0, h)
	return ui.Rect{X: x0, Y: y0, W: x1 - x0, H: y1 - y0}
}

// cropRect maps a selection in DIPs onto the pixels of the display's image.
func cropRect(sel, content ui.Rect, img image.Rectangle) image.Rectangle {
	if content.W <= 0 || content.H <= 0 || img.Empty() || sel.W <= 0 || sel.H <= 0 {
		return image.Rectangle{}
	}
	sx := float64(img.Dx()) / float64(content.W)
	sy := float64(img.Dy()) / float64(content.H)
	x0 := int(math.Round(float64(sel.X) * sx))
	y0 := int(math.Round(float64(sel.Y) * sy))
	x1 := int(math.Round(float64(sel.X+sel.W) * sx))
	y1 := int(math.Round(float64(sel.Y+sel.H) * sy))
	if x0 < 0 {
		x0 = 0
	}
	if y0 < 0 {
		y0 = 0
	}
	if x1 > img.Dx() {
		x1 = img.Dx()
	}
	if y1 > img.Dy() {
		y1 = img.Dy()
	}
	if x1 <= x0 || y1 <= y0 {
		return image.Rectangle{}
	}
	return image.Rect(img.Min.X+x0, img.Min.Y+y0, img.Min.X+x1, img.Min.Y+y1)
}

// pixelSize is the size of a selection in the display's own pixels, as the
// label beside it says it.
func pixelSize(sel, content ui.Rect, img image.Rectangle) (int, int) {
	r := cropRect(sel, content, img)
	return r.Dx(), r.Dy()
}

// shotAction is what a button of the overlay does.
type shotAction uint8

const (
	actCopy shotAction = iota
	actSave
	actCancel
)

// shotButton is one button drawn over the picture.
type shotButton struct {
	action shotAction
	label  string
	rect   ui.Rect // local to the overlay's content
}

// buttonsAt lays the toolbar out in the bottom-right corner of a w×h area.
func buttonsAt(w, h float32) []shotButton {
	const (
		height = 30
		gap    = 8
		margin = 16
	)
	labels := []struct {
		action shotAction
		label  string
	}{
		{actCopy, "复制"},
		{actSave, "保存"},
		{actCancel, "取消"},
	}
	width := float32(64)
	gapTotal := float32(len(labels)-1) * gap
	if w-2*margin < width*float32(len(labels))+gapTotal {
		width = (w - 2*margin - gapTotal) / float32(len(labels))
		// Keep the row inside the margins even after the float math:
		// rounding up by a hundredth of a DIP is invisible, but it is
		// enough to draw past the edge.
		width = float32(math.Floor(float64(width)*100)) / 100
		if width < 32 {
			width = 32
		}
	}
	total := width*float32(len(labels)) + gapTotal
	x := w - margin - total
	if x < margin {
		x = margin
	}
	y := h - margin - height
	if y < margin {
		y = margin
	}
	out := make([]shotButton, 0, len(labels))
	for i, l := range labels {
		out = append(out, shotButton{
			action: l.action,
			label:  l.label,
			rect:   ui.Rect{X: x + float32(i)*(width+gap), Y: y, W: width, H: height},
		})
	}
	return out
}

// saveName is the name the save dialog starts with.
func saveName(t time.Time) string {
	return "screenshot-" + t.Format("20060102-150405") + ".png"
}

// savePath is the file the save dialog starts at: the pictures folder when
// the desktop has one, the home folder otherwise.
func savePath(now time.Time) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return saveName(now)
	}
	dir := filepath.Join(home, "Pictures")
	if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
		dir = home
	}
	return filepath.Join(dir, saveName(now))
}

func abs(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}

func clamp32(v, lo, hi float32) float32 {
	return min(max(v, lo), hi)
}
