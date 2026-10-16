package tools

import (
	"image"
	"image/color"
	"strings"

	"github.com/boombuler/barcode"
	"github.com/boombuler/barcode/qr"
	"github.com/egoist/mygo/ui"

	"gitee.com/yinshuwei/mytools/internal/widgets"
)

// QR makes a QR code of a text.
type QR struct {
	text  string
	level string // L, M, Q or H
	size  float64

	bitmap   *ui.Bitmap
	genText  string
	genLevel string
	err      string
}

func (*QR) ID() string   { return "qr" }
func (*QR) Name() string { return "二维码" }
func (*QR) Desc() string { return "把文本生成二维码" }

// levels are the error correction levels the select offers.
var levels = []string{"L", "M", "Q", "H"}

// sync encodes the text again when it or the level changed.
func (q *QR) sync() {
	if q.level == "" {
		q.level = "M"
	}
	if q.size <= 0 {
		q.size = 240
	}
	if q.text == q.genText && q.level == q.genLevel {
		return
	}
	q.genText, q.genLevel = q.text, q.level
	content := strings.TrimSpace(q.text)
	if content == "" {
		q.bitmap, q.err = nil, ""
		return
	}
	level := map[string]qr.ErrorCorrectionLevel{"L": qr.L, "M": qr.M, "Q": qr.Q, "H": qr.H}[q.level]
	code, err := qr.Encode(content, level, qr.Auto)
	if err != nil {
		q.bitmap, q.err = nil, err.Error()
		return
	}
	q.bitmap, q.err = ui.NewBitmap(qrImage(code, 4, int(q.size))), ""
}

// qrImage draws the code on white, with a quiet zone of quiet modules and
// each module as a square of scale pixels, so that it stays crisp.
func qrImage(bc barcode.Barcode, quiet, target int) image.Image {
	b := bc.Bounds()
	w, h := b.Dx(), b.Dy()
	n := w + quiet*2
	scale := 1
	if n > 0 && target/n > 1 {
		scale = target / n
	}
	img := image.NewRGBA(image.Rect(0, 0, n*scale, n*scale))
	for i := range img.Pix {
		img.Pix[i] = 0xff // opaque white
	}
	black := color.RGBA{A: 255}
	for y := range h {
		for x := range w {
			r, g, bl, _ := bc.At(b.Min.X+x, b.Min.Y+y).RGBA()
			if r != 0 || g != 0 || bl != 0 {
				continue
			}
			for dy := range scale {
				for dx := range scale {
					img.SetRGBA((x+quiet)*scale+dx, (y+quiet)*scale+dy, black)
				}
			}
		}
	}
	return img
}

func (q *QR) View(c *ui.Context) {
	t := c.Theme()
	q.sync()

	widgets.Field(c, "内容", func() {
		ui.TextArea(c.Key("text"), &q.text).Label("二维码内容").Placeholder("https://example.com").Lines(3, 8).FillWidth()
	})
	ui.Row(c).Gap(8).Wrap().AlignItems(ui.Center).Children(func() {
		widgets.Hint(c, "纠错级别")
		ui.Select(c.Key("level"), &q.level, levels).Width(90)
		widgets.Hint(c, "尺寸")
		ui.Slider(c.Key("size"), &q.size, 120, 420).Step(10).Width(180)
		ui.Textf(c, "%.0f px", q.size).FontSize(12).TextColor(t.TextMuted)
		ui.Spacer(c)
		if ui.Button(c.Key("clear"), "清空").Clicked() {
			q.text = ""
		}
	})
	if q.err != "" {
		ui.Text(c, "错误："+q.err).FontSize(12).TextColor(t.Danger)
	}
	ui.Column(c).PaddingY(8).Center().FillWidth().Children(func() {
		if q.bitmap != nil {
			ui.Image(c.Key("code"), q.bitmap)
		} else {
			widgets.Hint(c, "输入内容后自动生成二维码")
		}
	})
}
