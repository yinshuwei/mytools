// Package shots takes a region of the screen, lets the user choose and
// annotate nothing yet, then copies or saves it.
package shots

import (
	"bytes"
	"errors"
	"image/png"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/transfer"

	"github.com/yinshuwei/mytools/internal/capture"
)

// hideDelay lets the toolbox window leave the screen before the picture is
// taken: a window that is still fading out would be in it.
const hideDelay = 160 * time.Millisecond

// Service runs the screenshot: hiding the toolbox, taking the picture,
// opening an overlay on every display, and putting the chosen part on the
// clipboard or in a file.
type Service struct {
	win     *mygo.Window
	problem error // why a screenshot cannot be taken here, nil when it can

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
	log.Printf("shots: start busy=%v finishing=%v problem=%v", s.busy, s.finishing, s.problem)
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
	log.Printf("shots: captured %d frames err=%v", len(frames), err)
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
	log.Printf("shots: %d overlays up", len(s.overlays))
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
	log.Printf("shots: cancel busy=%v finishing=%v", s.busy, s.finishing)
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

func (s *Service) copy(o *overlay) { s.finish(o, true, false) }
func (s *Service) save(o *overlay) { s.finish(o, false, true) }

// finish takes the selected pixels, closes the overlays, and lets the
// toolbox come back while the picture is encoded and handed over.
func (s *Service) finish(o *overlay, copyIt, saveIt bool) {
	log.Printf("shots: finish copy=%v save=%v sel=%+v size=%+v", copyIt, saveIt, o.sel, o.size)
	if !s.busy || s.finishing || !o.hasSelection() {
		return
	}
	rect := cropRect(o.sel, o.size, o.frame.Image.Bounds())
	if rect.Empty() {
		return
	}
	s.finishing = true
	img := o.frame.Image.SubImage(rect)
	// Same reason as cancel: close the overlays after this event has been
	// dispatched, not inside it.
	s.win.Update(func() {
		s.closeOverlays()
		s.restore()
		s.busy = false
		s.finishing = false

		go func() {
			var buf bytes.Buffer
			if err := png.Encode(&buf, img); err != nil {
				s.report(err)
				return
			}
			data := buf.Bytes()
			if copyIt {
				if err := copyPNG(data); err != nil {
					s.report(err)
				}
			}
			if saveIt {
				s.savePNG(data)
			}
		}()
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

func copyPNG(data []byte) error {
	return mygo.Clipboard.Write(transfer.New(transfer.NewItem(transfer.Bytes(transfer.PNG, data))))
}

func (s *Service) savePNG(data []byte) {
	path, err := mygo.Dialog.Save(mygo.SaveDialogOptions{
		Parent:      s.win,
		Title:       "保存截图",
		DefaultPath: savePath(time.Now()),
		Filters:     []mygo.FileFilter{{Name: "PNG 图片", Extensions: []string{"png"}}},
	})
	if err != nil {
		s.report(err)
		return
	}
	if path == "" {
		return // the dialog was dismissed
	}
	if !strings.HasSuffix(strings.ToLower(path), ".png") {
		path += ".png"
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		s.report(err)
	}
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

// SaveDir names the folder the save dialog starts in, for tests and for
// anything that wants to show it.
func SaveDir() string { return filepath.Dir(savePath(time.Now())) }
