//go:build linux && (amd64 || arm64)

package capture

import (
	"image"
	"testing"
)

// TestXImageImageBGRX checks the common case: a little-endian 32-bit
// ZPixmap whose bytes are B, G, R, X.
func TestXImageImageBGRX(t *testing.T) {
	data := []byte{
		0x10, 0x20, 0x30, 0x00, // blue, green, red, unused
		0x40, 0x50, 0x60, 0xff,
	}
	xi := &xImage{
		Width:        2,
		Height:       1,
		Format:       zPixmap,
		Data:         &data[0],
		ByteOrder:    0, // LSBFirst
		BytesPerLine: 8,
		BitsPerPixel: 32,
		RedMask:      0x00ff0000,
		GreenMask:    0x0000ff00,
		BlueMask:     0x000000ff,
	}
	img, err := xImageImage(xi)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := img.Bounds(), image.Rect(0, 0, 2, 1); got != want {
		t.Fatalf("bounds: got %v, want %v", got, want)
	}
	want := []uint8{
		0x30, 0x20, 0x10, 0xff,
		0x60, 0x50, 0x40, 0xff,
	}
	for i, v := range want {
		if img.Pix[i] != v {
			t.Errorf("pixel byte %d: got %#x, want %#x", i, img.Pix[i], v)
		}
	}
}

// TestXImageImageStride checks that padding at the end of a row is skipped
// and that rows are copied in order.
func TestXImageImageStride(t *testing.T) {
	data := []byte{
		0x01, 0x02, 0x03, 0x00, 0x04, 0x05, 0x06, 0x00, 0xaa, 0xaa, // padding
		0x07, 0x08, 0x09, 0x00, 0x0a, 0x0b, 0x0c, 0x00, 0xbb, 0xbb, // padding
	}
	xi := &xImage{
		Width:        2,
		Height:       2,
		Format:       zPixmap,
		Data:         &data[0],
		BytesPerLine: 10,
		BitsPerPixel: 32,
		RedMask:      0x00ff0000,
		GreenMask:    0x0000ff00,
		BlueMask:     0x000000ff,
	}
	img, err := xImageImage(xi)
	if err != nil {
		t.Fatal(err)
	}
	if got := img.RGBAAt(0, 0); got.R != 0x03 || got.G != 0x02 || got.B != 0x01 {
		t.Errorf("(0,0): got %v", got)
	}
	if got := img.RGBAAt(1, 1); got.R != 0x0c || got.G != 0x0b || got.B != 0x0a {
		t.Errorf("(1,1): got %v", got)
	}
}

func TestXImageImageRejects(t *testing.T) {
	good := []byte{0, 0, 0, 0}
	cases := []struct {
		name string
		xi   *xImage
	}{
		{"nil", nil},
		{"no data", &xImage{Width: 1, Height: 1, Format: zPixmap}},
		{"empty", &xImage{Format: zPixmap, Data: &good[0]}},
		{"bitmap format", &xImage{Width: 1, Height: 1, Format: 0, Data: &good[0], BitsPerPixel: 32}},
		{"short stride", &xImage{Width: 2, Height: 1, Format: zPixmap, Data: &good[0], BitsPerPixel: 32, BytesPerLine: 4}},
		{"odd depth", &xImage{Width: 1, Height: 1, Format: zPixmap, Data: &good[0], BitsPerPixel: 8, BytesPerLine: 4}},
	}
	for _, c := range cases {
		if _, err := xImageImage(c.xi); err == nil {
			t.Errorf("%s: expected an error", c.name)
		}
	}
}
