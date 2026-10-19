package tools

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"strconv"
	"strings"

	"github.com/boombuler/barcode"
	"github.com/boombuler/barcode/qr"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/transfer"
	"github.com/egoist/mygo/ui"
	"github.com/makiuchi-d/gozxing"
	"github.com/makiuchi-d/gozxing/qrcode"
	xdraw "golang.org/x/image/draw"

	"gitee.com/yinshuwei/mytools/internal/widgets"
)

// QR makes a QR code of a text, with room for a logo in its middle, and
// reads one back from an image.
type QR struct {
	win  *mygo.Window
	mode int // 0 生成, 1 解码

	level string // L, M, Q or H
	size  float64
	text  string

	logo    string // the logo's path, "" for none
	logoImg image.Image
	logoErr string

	code      *ui.Bitmap
	codeImage image.Image
	generated string // what the code is of
	err       string

	decoded   string
	decodeErr string
	busy      bool

	logoPick   chan pick
	decodePick chan pick
}

func (*QR) ID() string   { return "qr" }
func (*QR) Name() string { return "二维码" }
func (*QR) Desc() string { return "生成二维码（可带 Logo）、从图片里读二维码" }

// SetWindow is what WindowAware asks of a tool acting on the window.
func (q *QR) SetWindow(win *mygo.Window) { q.win = win }

// levels are the error correction levels the select offers.
var levels = []string{"L", "M", "Q", "H"}

// logoRatio is how much of the code's side a logo covers.
const logoRatio = 0.24

// quietModules is the white margin around the code, in modules. The
// standard asks for four, which at a large size reads as a wide frame
// around the code; two scans as well and leaves more of the picture to the
// code itself.
const quietModules = 2

// genKey is what the code is of, so that changing any of it makes the code
// again: the size takes effect as it is dragged.
func (q *QR) genKey() string {
	return strings.Join([]string{q.text, q.level, strconv.Itoa(int(q.size)), q.logo}, "\x00")
}

// sync encodes the text again when anything the code is of changed.
func (q *QR) sync() {
	if q.level == "" {
		q.level = "M"
	}
	if q.size <= 0 {
		q.size = 240
	}
	key := q.genKey()
	if key == q.generated {
		return
	}
	q.generated = key
	content := strings.TrimSpace(q.text)
	if content == "" {
		q.code, q.codeImage, q.err = nil, nil, ""
		return
	}
	level := map[string]qr.ErrorCorrectionLevel{"L": qr.L, "M": qr.M, "Q": qr.Q, "H": qr.H}[q.level]
	code, err := qr.Encode(content, level, qr.Auto)
	if err != nil {
		q.code, q.codeImage, q.err = nil, nil, err.Error()
		return
	}
	img := qrImage(code, quietModules, int(q.size))
	if q.logoImg != nil {
		drawLogo(img, q.logoImg)
	}
	q.code, q.codeImage, q.err = ui.NewBitmap(img), img, ""
}

// qrImage draws the code on white, with a quiet zone of quiet modules and
// each module as a square of scale pixels, so that it stays crisp.
func qrImage(bc barcode.Barcode, quiet, target int) *image.RGBA {
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

// drawLogo puts logo in the middle of img, over a white patch so that what
// is left of the code around it still reads.
func drawLogo(img *image.RGBA, logo image.Image) {
	b := img.Bounds()
	side := int(float64(b.Dx()) * logoRatio)
	if side < 8 || logo == nil {
		return
	}
	at := image.Rect(0, 0, side, side).Add(image.Pt(
		b.Min.X+(b.Dx()-side)/2, b.Min.Y+(b.Dy()-side)/2))
	xdraw.Draw(img, at.Inset(-side/6), image.NewUniform(color.White), image.Point{}, xdraw.Src)
	xdraw.CatmullRom.Scale(img, at, logo, logo.Bounds(), xdraw.Over, nil)
}

// copyCode puts the code on the clipboard as a picture, ready to paste into
// a chat as it is.
func (q *QR) copyCode(c *ui.Context) {
	if q.codeImage == nil {
		return
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, q.codeImage); err != nil {
		q.err = "复制失败：" + err.Error()
		return
	}
	c.Services().WriteClipboardData(transfer.New(transfer.NewItem(
		transfer.Bytes(transfer.PNG, buf.Bytes()),
	)))
	c.Toast("已复制二维码图片")
}

// decodeQR reads the QR code of an image.
func decodeQR(img image.Image) (string, error) {
	bmp, err := gozxing.NewBinaryBitmapFromImage(img)
	if err != nil {
		return "", err
	}
	res, err := qrcode.NewQRCodeReader().Decode(bmp, map[gozxing.DecodeHintType]interface{}{
		gozxing.DecodeHintType_TRY_HARDER: true,
	})
	if err != nil {
		return "", err
	}
	return res.GetText(), nil
}

// applyLogo takes what the file dialog returned for the logo.
func (q *QR) applyLogo(p pick) {
	switch {
	case p.err != nil:
		q.logoErr = p.err.Error()
	case p.path == "":
		// Canceled.
	default:
		img, _, err := image.Decode(bytes.NewReader(p.data))
		if err != nil {
			q.logoErr = err.Error()
			return
		}
		q.logo, q.logoImg, q.logoErr = p.path, img, ""
	}
}

// applyDecode takes what the file dialog returned to be read.
func (q *QR) applyDecode(p pick) {
	q.busy = false
	switch {
	case p.err != nil:
		q.decodeErr = p.err.Error()
	case p.path == "":
		// Canceled.
	default:
		text, err := decodeImage(p.data)
		if err != nil {
			q.decoded, q.decodeErr = "", "没读出二维码："+err.Error()
			return
		}
		q.decoded, q.decodeErr = text, ""
	}
}

// decodeImage reads the QR code of an image's bytes.
func decodeImage(data []byte) (string, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	return decodeQR(img)
}

// readOne reads the code the app itself made.
func (q *QR) readOne() {
	if q.codeImage == nil {
		return
	}
	text, err := decodeQR(q.codeImage)
	if err != nil {
		q.decoded, q.decodeErr = "", "没读出二维码："+err.Error()
		return
	}
	q.decoded, q.decodeErr = text, ""
}

func (q *QR) View(c *ui.Context) {
	t := c.Theme()
	if q.logoPick == nil {
		q.logoPick, q.decodePick = make(chan pick, 4), make(chan pick, 4)
	}
	widgets.Drain(q.logoPick, q.applyLogo)
	widgets.Drain(q.decodePick, q.applyDecode)
	q.sync()

	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		widgets.Caption(c, "二维码")
		ui.Segmented(c.Key("mode"), &q.mode, "生成", "解码")
	})
	if q.mode == 0 {
		q.makeView(c, t)
	} else {
		q.readView(c, t)
	}
}

// makeView is the page that writes a code.
func (q *QR) makeView(c *ui.Context, t *ui.Theme) {
	widgets.Field(c, "内容", func() {
		ui.TextArea(c.Key("text"), &q.text).Label("二维码内容").
			Placeholder("https://example.com").Lines(3, 8).FillWidth()
	})
	ui.Row(c).Gap(8).Wrap().AlignItems(ui.Center).Children(func() {
		widgets.Hint(c, "纠错级别")
		ui.Select(c.Key("level"), &q.level, levels).Width(80)
		widgets.Hint(c, "尺寸")
		ui.Slider(c.Key("size"), &q.size, 120, 1000).Step(10).Width(160)
		ui.Textf(c, "%.0f px", q.size).FontSize(widgets.Scaled(12)).TextColor(t.TextMuted)
		ui.Spacer(c)
		logo := ui.Button(c.Key("logo"), "选择 Logo…").
			Tooltip("在二维码中间放一张图片（" + widgets.ShortcutHint(0, "O") + "）")
		if filePressed(c, logo) {
			pickFile(q.win, c.Services(), "选择 Logo", imageFilters, q.logoPick)
		}
		if q.logo != "" {
			if ui.Button(c.Key("clearlogo"), "去掉 Logo").Clicked() {
				q.logo, q.logoImg, q.logoErr = "", nil, ""
			}
		}
		if ui.Button(c.Key("clear"), "清空").Clicked() {
			q.text = ""
		}
	})
	if q.logo != "" && q.level != "Q" && q.level != "H" {
		ui.Text(c, "带 Logo 时中间会被挡住，建议纠错级别选 Q 或 H，否则可能扫不出来").
			FontSize(12).TextColor(t.Warning)
	}
	if q.logoErr != "" {
		ui.Text(c, "Logo 错误："+q.logoErr).FontSize(widgets.Scaled(12)).TextColor(t.Danger)
	}
	if q.err != "" {
		ui.Text(c, "错误："+q.err).FontSize(widgets.Scaled(12)).TextColor(t.Danger)
	}
	ui.Column(c).PaddingY(8).Center().FillWidth().Children(func() {
		if q.code != nil {
			ui.Image(c.Key("code"), q.code)
		} else {
			widgets.Hint(c, "输入内容后自动生成二维码")
		}
	})
	ui.Row(c).Gap(8).Wrap().AlignItems(ui.Center).Children(func() {
		copyImg := ui.PrimaryButton(c.Key("copyimg"), "复制图片").Disabled(q.code == nil).
			Tooltip("把二维码图片放进剪贴板（" + widgets.ShortcutHint(0, "Enter") + "）")
		if q.code != nil && mainPressed(c, copyImg) {
			q.copyCode(c)
		}
		widgets.Hint(c, "复制的是图片本身，可直接粘贴到微信、文档或聊天窗口")
	})
}

// readView is the page that reads a code back.
func (q *QR) readView(c *ui.Context, t *ui.Theme) {
	ui.Row(c).Gap(8).Wrap().AlignItems(ui.Center).Children(func() {
		pick := ui.PrimaryButton(c.Key("pick"), "选择图片…").Disabled(q.busy).
			Tooltip("选一张带二维码的图片（" + widgets.ShortcutHint(0, "O") + "）")
		if !q.busy && (filePressed(c, pick) || mainPressed(c, pick)) {
			q.busy = true
			pickFile(q.win, c.Services(), "选择二维码图片", imageFilters, q.decodePick)
		}
		if ui.Button(c.Key("pickcode"), "读刚生成的码").Disabled(q.codeImage == nil).
			Tooltip("把「生成」里当前这个码读一遍，看能不能扫出来").Clicked() {
			q.readOne()
		}
		if q.busy {
			widgets.Hint(c, "等待选择…")
		}
		ui.Spacer(c)
		widgets.Hint(c, "支持 PNG / JPEG / GIF / WebP / BMP")
	})
	if q.decodeErr != "" {
		ui.Text(c, "错误："+q.decodeErr).FontSize(widgets.Scaled(12)).TextColor(t.Danger)
	}
	// Keyed: the line reporting an error comes and goes above it.
	widgets.FieldKey(c, "decodedfield", "读出的内容", func() {
		widgets.Result(c, "decoded", "解码结果", &q.decoded, 4, 14)
	})
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		widgets.CopyButton(c.Key("copy"), "复制结果", q.decoded)
		if ui.Button(c.Key("togen"), "拿去生成").Disabled(q.decoded == "").Clicked() {
			q.text, q.mode = q.decoded, 0
		}
	})
}
