// Package capture takes a picture of each display, for the screenshot tool.
// It talks to the windowing system itself, with no cgo, so that the same
// source builds for every target of the toolbox.
package capture

import (
	"errors"
	"fmt"
	"image"

	"github.com/egoist/mygo"
)

// Display is a monitor to capture, in the coordinates the window system
// places windows in: DIPs, whose origin is the top-left of the primary
// display.
type Display struct {
	// ID is mygo.Display.ID, which the platform code uses to find the
	// monitor it belongs to.
	ID int64
	// X, Y, Width and Height are the display's bounds in DIPs.
	X, Y, Width, Height int
	// Scale is how many screen pixels a DIP is, as mygo reports it.
	Scale float64
}

// Rect is the display's screen area in DIPs.
func (d Display) Rect() image.Rectangle {
	return image.Rect(d.X, d.Y, d.X+d.Width, d.Y+d.Height)
}

// A Frame is one display's picture, at the display's own resolution.
type Frame struct {
	Display Display
	Image   *image.RGBA
}

// Errors the windows of the toolbox say in words of their own.
var (
	// ErrUnsupported means screen capture is not implemented here.
	ErrUnsupported = errors.New("这个平台还不支持截屏")
	// ErrWayland means the session is Wayland, where X11 capture sees no
	// native windows.
	ErrWayland = errors.New("Wayland 会话下暂不支持截屏，登录 X11 会话后可以用")
	// ErrPermission means macOS has not let the app record the screen.
	ErrPermission = errors.New("还没有屏幕录制权限")
)

// Available reports why the screen cannot be captured, nil when it can.
// It is cheap enough to ask as a page is drawn, and does not prompt for a
// permission.
func Available() error { return available() }

// FromDisplay turns a display mygo knows into one to capture.
func FromDisplay(d mygo.Display) Display {
	return Display{
		ID:     d.ID,
		X:      d.Bounds.X,
		Y:      d.Bounds.Y,
		Width:  d.Bounds.Width,
		Height: d.Bounds.Height,
		Scale:  d.ScaleFactor,
	}
}

// Capture takes a picture of every display, in the order given. A display
// that cannot be captured fails the whole shot: a picture of some of the
// screen would be worse than none.
func Capture(displays []Display) ([]Frame, error) {
	if len(displays) == 0 {
		return nil, errors.New("没有找到显示器")
	}
	images, err := captureDisplays(displays)
	if err != nil {
		return nil, err
	}
	if len(images) != len(displays) {
		return nil, fmt.Errorf("截屏返回了 %d 块屏幕，expected %d", len(images), len(displays))
	}
	frames := make([]Frame, len(displays))
	for i, img := range images {
		if img == nil || img.Bounds().Empty() {
			return nil, fmt.Errorf("屏幕 %d 的截图为空", i+1)
		}
		frames[i] = Frame{Display: displays[i], Image: img}
	}
	return frames, nil
}
