//go:build darwin && (amd64 || arm64)

package capture

import (
	"errors"
	"fmt"
	"image"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
)

// cgRect is CGRect: four CGFloats, passed by value.
type cgRect struct {
	X, Y, Width, Height float64
}

var (
	cgOnce sync.Once
	cgErr  error

	cgDisplayCreateImage           func(uint32) uintptr
	cgImageGetWidth                func(uintptr) uintptr
	cgImageGetHeight               func(uintptr) uintptr
	cgImageRelease                 func(uintptr)
	cgColorSpaceCreateDeviceRGB    func() uintptr
	cgColorSpaceRelease            func(uintptr)
	cgBitmapContextCreate          func(unsafe.Pointer, uintptr, uintptr, uintptr, uintptr, uintptr, uint32) uintptr
	cgContextDrawImage             func(uintptr, cgRect, uintptr)
	cgContextRelease               func(uintptr)
	cgPreflightScreenCaptureAccess func() bool
	cgRequestScreenCaptureAccess   func() bool
)

func loadCoreGraphics() error {
	cgOnce.Do(func() {
		lib, err := purego.Dlopen(
			"/System/Library/Frameworks/CoreGraphics.framework/CoreGraphics",
			purego.RTLD_NOW|purego.RTLD_GLOBAL,
		)
		if err != nil {
			cgErr = fmt.Errorf("找不到 CoreGraphics：%w", err)
			return
		}
		for _, b := range []struct {
			fn   any
			name string
		}{
			{&cgDisplayCreateImage, "CGDisplayCreateImage"},
			{&cgImageGetWidth, "CGImageGetWidth"},
			{&cgImageGetHeight, "CGImageGetHeight"},
			{&cgImageRelease, "CGImageRelease"},
			{&cgColorSpaceCreateDeviceRGB, "CGColorSpaceCreateDeviceRGB"},
			{&cgColorSpaceRelease, "CGColorSpaceRelease"},
			{&cgBitmapContextCreate, "CGBitmapContextCreate"},
			{&cgContextDrawImage, "CGContextDrawImage"},
			{&cgContextRelease, "CGContextRelease"},
		} {
			sym, err := purego.Dlsym(lib, b.name)
			if err != nil || sym == 0 {
				cgErr = fmt.Errorf("CoreGraphics 缺少 %s", b.name)
				return
			}
			purego.RegisterFunc(b.fn, sym)
		}
		// Screen recording access arrived in macOS 10.15; older systems
		// simply capture without asking.
		if sym, err := purego.Dlsym(lib, "CGPreflightScreenCaptureAccess"); err == nil && sym != 0 {
			purego.RegisterFunc(&cgPreflightScreenCaptureAccess, sym)
		}
		if sym, err := purego.Dlsym(lib, "CGRequestScreenCaptureAccess"); err == nil && sym != 0 {
			purego.RegisterFunc(&cgRequestScreenCaptureAccess, sym)
		}
	})
	return cgErr
}

func available() error { return loadCoreGraphics() }

func captureDisplays(displays []Display) ([]*image.RGBA, error) {
	if err := loadCoreGraphics(); err != nil {
		return nil, err
	}
	// Without the permission macOS hands back an empty or black picture.
	// Ask once, then say what to do: the system applies a new grant only
	// to a freshly started app.
	if cgPreflightScreenCaptureAccess != nil && !cgPreflightScreenCaptureAccess() {
		if cgRequestScreenCaptureAccess != nil {
			cgRequestScreenCaptureAccess()
		}
		return nil, ErrPermission
	}
	out := make([]*image.RGBA, 0, len(displays))
	for _, d := range displays {
		img, err := captureDisplay(uint32(d.ID))
		if err != nil {
			return nil, fmt.Errorf("读取显示器 %q：%w", displayName(d), err)
		}
		out = append(out, img)
	}
	return out, nil
}

func displayName(d Display) string {
	return fmt.Sprintf("%d,%d %d×%d", d.X, d.Y, d.Width, d.Height)
}

func captureDisplay(id uint32) (*image.RGBA, error) {
	img := cgDisplayCreateImage(id)
	if img == 0 {
		return nil, errors.New("系统没有返回画面")
	}
	defer cgImageRelease(img)

	w := int(cgImageGetWidth(img))
	h := int(cgImageGetHeight(img))
	if w <= 0 || h <= 0 {
		return nil, errors.New("系统返回了空尺寸的画面")
	}

	// Draw into a bitmap of known layout (RGBA, big-endian components),
	// which spares knowing what format CGDisplayCreateImage chose today.
	pix := make([]byte, w*h*4)
	cs := cgColorSpaceCreateDeviceRGB()
	if cs == 0 {
		return nil, errors.New("建立颜色空间失败")
	}
	defer cgColorSpaceRelease(cs)
	const (
		alphaPremultipliedLast = 1
		byteOrder32Big         = 4 << 12
	)
	ctx := cgBitmapContextCreate(unsafe.Pointer(&pix[0]), uintptr(w), uintptr(h), 8, uintptr(w*4), cs, alphaPremultipliedLast|byteOrder32Big)
	if ctx == 0 {
		return nil, errors.New("建立位图上下文失败")
	}
	defer cgContextRelease(ctx)
	cgContextDrawImage(ctx, cgRect{X: 0, Y: 0, Width: float64(w), Height: float64(h)}, img)

	out := &image.RGBA{Pix: pix, Stride: w * 4, Rect: image.Rect(0, 0, w, h)}
	for i := 3; i < len(pix); i += 4 {
		pix[i] = 0xff // the screen is opaque
	}
	return out, nil
}
