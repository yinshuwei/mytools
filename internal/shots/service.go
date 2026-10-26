// Package shots takes a region of the screen and hands it to the toolbox,
// where it is edited and annotated and then copied or saved.
package shots

import (
	"errors"
	"image"
	"image/draw"
	"time"

	"github.com/egoist/mygo"

	"github.com/yinshuwei/mytools/internal/capture"
)

// hideDelay lets the toolbox window leave the screen before the picture is
// taken: a window that is still fading out would be in it.
const hideDelay = 160 * time.Millisecond

// Service runs the screenshot: hiding the toolbox, taking the picture,
// opening an overlay on every display, and handing the chosen part to the
// toolbox to edit.
type Service struct {
	win     *mygo.Window
	problem error // why a screenshot cannot be taken here, nil when it can

	// Deliver takes the chosen picture and the display's scale, in pixels
	// per DIP. It runs on the main thread, once the overlays are gone and
	// the toolbox is back.
	Deliver func(img *image.RGBA, scale float64)

	busy      bool
	finishing bool
	hidden    bool
	gen       int
	overlays  []*overlay
}

// New makes the service for the toolbox's window.
func New(win *mygo.Window) *Service {
	return &Service{win: win, problem: capture.Available()}
}

// Problem is why screenshots cannot be taken, nil when they can.
func (s *Service) Problem() error { return s.problem }

// Start takes a screenshot. It returns at once; the overlays come up when
// the picture is ready. It is safe to call from a menu, a button, or a
// global shortcut.
func (s *Service) Start() {
	if s.win == nil || s.win.IsDestroyed() {
		return
	}
	// Everything the service owns is touched on the main thread only:
	// Start may be called by a shortcut callback on another one.
	s.win.Update(func() { s.start() })
}

func (s *Service) start() {
	if s.busy || s.finishing || s.win.IsDestroyed() {
		return
	}
	if s.problem != nil {
		s.report(s.problem)
		return
	}
	s.busy = true
	s.hidden = false
	s.gen++
	gen := s.gen
	if s.win.IsVisible() && !s.win.IsMinimized() {
		s.win.Hide()
		s.hidden = true
	}
	cursor := mygo.Screen.CursorScreenPoint()
	focus := mygo.Screen.DisplayNearestPoint(cursor).ID
	go s.capture(gen, focus)
}

func (s *Service) capture(gen int, focus int64) {
	if s.hidden {
		time.Sleep(hideDelay)
	}
	displays := mygo.Screen.Displays()
	targets := make([]capture.Display, 0, len(displays))
	for _, d := range displays {
		targets = append(targets, capture.FromDisplay(d))
	}
	frames, err := capture.Capture(targets)
	s.win.Update(func() {
		if s.gen != gen || !s.busy {
			return // superseded, or left while the picture was coming
		}
		if err != nil {
			s.fail(err)
			return
		}
		s.open(frames, focus)
	})
}

func (s *Service) open(frames []capture.Frame, focus int64) {
	for _, f := range frames {
		o := newOverlay(s, f)
		o.create()
		if o.win == nil || o.win.IsDestroyed() {
			continue
		}
		s.overlays = append(s.overlays, o)
	}
	if len(s.overlays) == 0 {
		s.fail(errors.New("没有截到任何屏幕"))
		return
	}
	for _, o := range s.overlays {
		o.win.Show()
	}
	for _, o := range s.overlays {
		if o.frame.Display.ID == focus {
			o.win.Focus()
			break
		}
	}
}

// fail closes what came up, puts the toolbox back, and says what happened.
func (s *Service) fail(err error) {
	s.closeOverlays()
	s.restore()
	s.busy = false
	s.finishing = false
	s.report(err)
}

// cancel leaves the overlays without a picture.
func (s *Service) cancel() {
	if !s.busy || s.finishing {
		return
	}
	s.finishing = true
	// Destroying the window whose callback is running, and raising the
	// main window from inside a key or pointer event, upsets GTK. The
	// teardown waits for the next turn of the loop.
	s.win.Update(func() {
		s.closeOverlays()
		s.restore()
		s.busy = false
		s.finishing = false
	})
}

// editOverlay takes the chosen pixels to the toolbox: the overlays go, the
// toolbox comes back, and the page there shows the picture to work on.
func (s *Service) editOverlay(o *overlay) {
	if !s.busy || s.finishing || !o.hasSelection() {
		return
	}
	rect := cropRect(o.sel, o.size, o.frame.Image.Bounds())
	if rect.Empty() {
		return
	}
	// The picture is copied out of the display's frame: it outlives the
	// overlay windows, and its own bounds make the annotations' coordinates
	// simple.
	img := image.NewRGBA(image.Rect(0, 0, rect.Dx(), rect.Dy()))
	draw.Draw(img, img.Bounds(), o.frame.Image, rect.Min, draw.Src)
	scale := o.frame.Display.Scale
	s.finishing = true
	// Same reason as cancel: the windows are closed after this event has
	// been dispatched, not inside it.
	s.win.Update(func() {
		s.closeOverlays()
		s.restore()
		s.busy = false
		s.finishing = false
		if s.Deliver != nil {
			s.Deliver(img, scale)
		}
	})
}

func (s *Service) closeOverlays() {
	for _, o := range s.overlays {
		if o.win != nil && !o.win.IsDestroyed() {
			o.win.Destroy()
		}
	}
	s.overlays = nil
}

func (s *Service) restore() {
	if !s.hidden {
		return
	}
	s.hidden = false
	s.win.Show()
	s.win.Focus()
}

func (s *Service) report(err error) {
	detail := Friendly(err)
	go func() {
		_, _ = mygo.Dialog.Message(mygo.MessageOptions{
			Parent:  s.win,
			Type:    mygo.MessageError,
			Message: "截图没有完成",
			Detail:  detail,
			Buttons: []string{"好"},
		})
	}()
}

// Friendly says in Chinese what a capture error means and what to do.
func Friendly(err error) string {
	switch {
	case errors.Is(err, capture.ErrWayland):
		return "当前是 Wayland 会话。截屏需要 X11：退出后在登录界面选择 X11 会话，再打开本应用。"
	case errors.Is(err, capture.ErrPermission):
		return "macOS 需要「屏幕录制」权限。打开系统设置 → 隐私与安全性 → 屏幕录制，勾选本应用，然后重新启动它再试。"
	case errors.Is(err, capture.ErrUnsupported):
		return "这个平台还不支持截屏。"
	}
	return err.Error()
}
