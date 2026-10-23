//go:build linux && (amd64 || arm64)

package capture

import (
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"math"
	"math/bits"
	"os"
	"strings"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
)

// X11 keeps a picture in an XImage: its pixels, and how to read them. The
// layout of the fields up to obdata has not changed since X11R6, and is the
// same on amd64 and arm64.
type xImage struct {
	Width, Height  int32
	Xoffset        int32
	Format         int32
	Data           *byte
	ByteOrder      int32
	BitmapUnit     int32
	BitmapBitOrder int32
	BitmapPad      int32
	Depth          int32
	BytesPerLine   int32
	BitsPerPixel   int32
	RedMask        uint64
	GreenMask      uint64
	BlueMask       uint64
	Obdata         uintptr
}

// XImage.Format is ZPixmap for the pixels XGetImage hands back; the other
// formats are bitmaps the screen never uses.
const zPixmap = 2

var (
	x11Once sync.Once
	x11Err  error

	xOpenDisplay   func(uintptr) uintptr
	xCloseDisplay  func(uintptr) int32
	xDefaultScreen func(uintptr) int32
	xRootWindow    func(uintptr, int32) uint64
	xDisplayWidth  func(uintptr, int32) int32
	xDisplayHeight func(uintptr, int32) int32
	xGetImage      func(uintptr, uint64, int32, int32, uint32, uint32, uint64, int32) unsafe.Pointer
	xDestroyImage  func(unsafe.Pointer) int32
)

func loadX11() error {
	x11Once.Do(func() {
		lib, err := purego.Dlopen("libX11.so.6", purego.RTLD_NOW|purego.RTLD_GLOBAL)
		if err != nil {
			lib, err = purego.Dlopen("libX11.so", purego.RTLD_NOW|purego.RTLD_GLOBAL)
		}
		if err != nil {
			x11Err = fmt.Errorf("找不到 libX11：%w", err)
			return
		}
		for _, b := range []struct {
			fn   any
			name string
		}{
			{&xOpenDisplay, "XOpenDisplay"},
			{&xCloseDisplay, "XCloseDisplay"},
			{&xDefaultScreen, "XDefaultScreen"},
			{&xRootWindow, "XRootWindow"},
			{&xDisplayWidth, "XDisplayWidth"},
			{&xDisplayHeight, "XDisplayHeight"},
			{&xGetImage, "XGetImage"},
			{&xDestroyImage, "XDestroyImage"},
		} {
			sym, err := purego.Dlsym(lib, b.name)
			if err != nil || sym == 0 {
				x11Err = fmt.Errorf("libX11 缺少 %s", b.name)
				return
			}
			purego.RegisterFunc(b.fn, sym)
		}
	})
	return x11Err
}

func available() error {
	if os.Getenv("WAYLAND_DISPLAY") != "" || strings.EqualFold(os.Getenv("XDG_SESSION_TYPE"), "wayland") {
		return ErrWayland
	}
	if os.Getenv("DISPLAY") == "" {
		return errors.New("没有可用的 X11 显示（DISPLAY 未设置）")
	}
	return loadX11()
}

func captureDisplays(displays []Display) ([]*image.RGBA, error) {
	if err := available(); err != nil {
		return nil, err
	}
	dpy := xOpenDisplay(0)
	if dpy == 0 {
		return nil, errors.New("连不上 X11 显示")
	}
	defer xCloseDisplay(dpy)
	screen := xDefaultScreen(dpy)
	root := xRootWindow(dpy, screen)
	rootW, rootH := int(xDisplayWidth(dpy, screen)), int(xDisplayHeight(dpy, screen))
	if rootW <= 0 || rootH <= 0 {
		return nil, errors.New("X11 显示没有尺寸")
	}
	out := make([]*image.RGBA, 0, len(displays))
	for _, d := range displays {
		scale := d.Scale
		if scale <= 0 {
			scale = 1
		}
		x := int(math.Round(float64(d.X) * scale))
		y := int(math.Round(float64(d.Y) * scale))
		w := int(math.Round(float64(d.Width) * scale))
		h := int(math.Round(float64(d.Height) * scale))
		// Keep the request inside the root window: XGetImage answers a
		// rectangle outside it with a BadMatch the default Xlib handler
		// treats as fatal.
		if x < 0 {
			w += x
			x = 0
		}
		if y < 0 {
			h += y
			y = 0
		}
		if x+w > rootW {
			w = rootW - x
		}
		if y+h > rootH {
			h = rootH - y
		}
		if w <= 0 || h <= 0 {
			return nil, fmt.Errorf("显示器 %q 不在 X11 根窗口里", displayName(d))
		}
		xi := xGetImage(dpy, root, int32(x), int32(y), uint32(w), uint32(h), ^uint64(0), zPixmap)
		if xi == nil {
			return nil, fmt.Errorf("读取显示器 %q 失败", displayName(d))
		}
		img, err := xImageImage((*xImage)(xi))
		xDestroyImage(xi)
		if err != nil {
			return nil, err
		}
		out = append(out, img)
	}
	return out, nil
}

func displayName(d Display) string {
	return fmt.Sprintf("%d,%d %d×%d", d.X, d.Y, d.Width, d.Height)
}

// xImageImage copies an XImage's pixels into an RGBA image, top row first.
func xImageImage(xi *xImage) (*image.RGBA, error) {
	if xi == nil || xi.Data == nil {
		return nil, errors.New("X11 返回了空的图像")
	}
	if xi.Format != zPixmap {
		return nil, fmt.Errorf("X11 返回了非 ZPixmap 图像（format %d）", xi.Format)
	}
	w, h := int(xi.Width), int(xi.Height)
	if w <= 0 || h <= 0 {
		return nil, errors.New("X11 返回了空尺寸的图像")
	}
	bpp := int(xi.BitsPerPixel)
	if bpp != 16 && bpp != 24 && bpp != 32 {
		return nil, fmt.Errorf("暂不支持 %d 位的 X11 图像", bpp)
	}
	stride := int(xi.BytesPerLine)
	if stride < (w*bpp+7)/8 {
		return nil, errors.New("X11 图像的行宽不对")
	}
	src := unsafe.Slice(xi.Data, stride*h)
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	step := bpp / 8

	// The usual screen: 32 bits a pixel, eight a channel, in the order
	// BGRX on a little-endian server, XRGB on a big-endian one.
	if bpp == 32 && xi.RedMask == 0x00ff0000 && xi.GreenMask == 0x0000ff00 && xi.BlueMask == 0x000000ff {
		little := xi.ByteOrder == 0
		for row := 0; row < h; row++ {
			s := src[row*stride:]
			d := img.Pix[row*img.Stride:]
			for x := 0; x < w; x++ {
				p := s[x*4:]
				if little {
					d[x*4+0] = p[2]
					d[x*4+1] = p[1]
					d[x*4+2] = p[0]
				} else {
					d[x*4+0] = p[0]
					d[x*4+1] = p[1]
					d[x*4+2] = p[2]
				}
				d[x*4+3] = 0xff
			}
		}
		return img, nil
	}

	var order binary.ByteOrder = binary.LittleEndian
	if xi.ByteOrder != 0 {
		order = binary.BigEndian
	}
	read := func(p []byte) uint64 {
		switch bpp {
		case 16:
			return uint64(order.Uint16(p))
		case 24:
			if order == binary.LittleEndian {
				return uint64(p[0]) | uint64(p[1])<<8 | uint64(p[2])<<16
			}
			return uint64(p[0])<<16 | uint64(p[1])<<8 | uint64(p[2])
		default:
			return uint64(order.Uint32(p))
		}
	}
	channel := func(v, mask uint64) uint8 {
		if mask == 0 {
			return 0
		}
		shift := uint(bits.TrailingZeros64(mask))
		max := mask >> shift
		n := (v & mask) >> shift
		if max == 0xff {
			return uint8(n)
		}
		return uint8(n * 255 / max)
	}
	for row := 0; row < h; row++ {
		s := src[row*stride:]
		d := img.Pix[row*img.Stride:]
		for x := 0; x < w; x++ {
			v := read(s[x*step:])
			d[x*4+0] = channel(v, xi.RedMask)
			d[x*4+1] = channel(v, xi.GreenMask)
			d[x*4+2] = channel(v, xi.BlueMask)
			d[x*4+3] = 0xff
		}
	}
	return img, nil
}
