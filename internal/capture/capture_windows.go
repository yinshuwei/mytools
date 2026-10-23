//go:build windows && (amd64 || arm64)

package capture

import (
	"errors"
	"fmt"
	"image"
	"sync"
	"syscall"
	"unsafe"
)

var (
	user32 = syscall.NewLazyDLL("user32.dll")
	gdi32  = syscall.NewLazyDLL("gdi32.dll")

	procEnumDisplayMonitors = user32.NewProc("EnumDisplayMonitors")
	procGetMonitorInfoW     = user32.NewProc("GetMonitorInfoW")
	procGetDC               = user32.NewProc("GetDC")
	procReleaseDC           = user32.NewProc("ReleaseDC")

	procCreateCompatibleDC = gdi32.NewProc("CreateCompatibleDC")
	procCreateDIBSection   = gdi32.NewProc("CreateDIBSection")
	procSelectObject       = gdi32.NewProc("SelectObject")
	procBitBlt             = gdi32.NewProc("BitBlt")
	procDeleteObject       = gdi32.NewProc("DeleteObject")
	procDeleteDC           = gdi32.NewProc("DeleteDC")
)

// The constants of GDI capture.
const (
	srcCopy    = 0x00cc0020
	captureBLT = 0x40000000
	dibRGB     = 0
)

type winRect struct {
	Left, Top, Right, Bottom int32
}

type monitorInfo struct {
	Size    uint32
	Monitor winRect
	Work    winRect
	Flags   uint32
}

type bitmapInfoHeader struct {
	Size          uint32
	Width         int32
	Height        int32
	Planes        uint16
	BitCount      uint16
	Compression   uint32
	SizeImage     uint32
	XPelsPerMeter int32
	YPelsPerMeter int32
	ClrUsed       uint32
	ClrImportant  uint32
}

type bitmapInfo struct {
	Header bitmapInfoHeader
	Colors [1]uint32
}

var (
	monitorMu   sync.Mutex
	monitorSink func(uintptr)
	monitorCB   = syscall.NewCallback(func(hmonitor, hdc, rect, data uintptr) uintptr {
		if monitorSink != nil {
			monitorSink(hmonitor)
		}
		return 1 // keep going
	})
)

func available() error { return nil }

func captureDisplays(displays []Display) ([]*image.RGBA, error) {
	monitorMu.Lock()
	defer monitorMu.Unlock()
	rects := make(map[int64]winRect)
	monitorSink = func(h uintptr) {
		var mi monitorInfo
		mi.Size = uint32(unsafe.Sizeof(mi))
		if r, _, _ := procGetMonitorInfoW.Call(h, uintptr(unsafe.Pointer(&mi))); r == 0 {
			return
		}
		rects[int64(h)] = mi.Monitor
	}
	if r, _, err := procEnumDisplayMonitors.Call(0, 0, monitorCB, 0); r == 0 {
		monitorSink = nil
		return nil, fmt.Errorf("枚举显示器失败：%w", err)
	}
	monitorSink = nil

	out := make([]*image.RGBA, 0, len(displays))
	for _, d := range displays {
		r, ok := rects[d.ID]
		if !ok {
			return nil, fmt.Errorf("找不到显示器 %q", displayName(d))
		}
		img, err := captureRect(r)
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

// captureRect copies a rectangle of the screen through GDI. The process is
// per-monitor DPI aware (mygo makes it so), hence the rectangle is in
// physical pixels and matches the monitor's own resolution.
func captureRect(r winRect) (*image.RGBA, error) {
	w, h := int(r.Right-r.Left), int(r.Bottom-r.Top)
	if w <= 0 || h <= 0 {
		return nil, errors.New("显示器没有尺寸")
	}
	screen, _, err := procGetDC.Call(0)
	if screen == 0 {
		return nil, fmt.Errorf("取不到屏幕设备上下文：%w", err)
	}
	defer procReleaseDC.Call(0, screen)

	mem, _, err := procCreateCompatibleDC.Call(screen)
	if mem == 0 {
		return nil, fmt.Errorf("建立内存设备上下文失败：%w", err)
	}
	defer procDeleteDC.Call(mem)

	bmi := bitmapInfo{Header: bitmapInfoHeader{
		Size:        uint32(unsafe.Sizeof(bitmapInfoHeader{})),
		Width:       int32(w),
		Height:      -int32(h), // top row first
		Planes:      1,
		BitCount:    32,
		Compression: dibRGB,
		SizeImage:   uint32(w * h * 4),
	}}
	var bits unsafe.Pointer
	bitmap, _, err := procCreateDIBSection.Call(
		mem,
		uintptr(unsafe.Pointer(&bmi)),
		dibRGB,
		uintptr(unsafe.Pointer(&bits)),
		0,
		0,
	)
	if bitmap == 0 || bits == nil {
		return nil, fmt.Errorf("建立位图失败：%w", err)
	}
	defer procDeleteObject.Call(bitmap)

	old, _, _ := procSelectObject.Call(mem, bitmap)
	defer procSelectObject.Call(mem, old)

	ok, _, err := procBitBlt.Call(
		mem, 0, 0, uintptr(w), uintptr(h),
		screen,
		uintptr(uint32(r.Left)), uintptr(uint32(r.Top)),
		srcCopy|captureBLT,
	)
	if ok == 0 {
		return nil, fmt.Errorf("BitBlt 失败：%w", err)
	}

	src := unsafe.Slice((*byte)(bits), w*h*4)
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for i, j := 0, 0; i < len(src); i, j = i+4, j+4 {
		img.Pix[j+0] = src[i+2]
		img.Pix[j+1] = src[i+1]
		img.Pix[j+2] = src[i+0]
		img.Pix[j+3] = 0xff
	}
	return img, nil
}
