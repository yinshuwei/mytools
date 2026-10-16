package tools

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	"image/png"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/transfer"
	"github.com/egoist/mygo/ui"

	"gitee.com/yinshuwei/mytools/internal/widgets"
)

// previewChars is how much of a long text the boxes show. A Base64 of a
// picture is megabytes on a single line, and laying out megabytes on every
// frame is what hangs the view; what one wants of a string that long is to
// copy it or save it, not to read it.
const previewChars = 600

// maxPreviewPixels is how large a picture may be and still be decoded for a
// preview. Past it the decode alone takes seconds and the pixels hundreds of
// megabytes, while what the tools are really for — the Base64, or the copy —
// goes on working.
const maxPreviewPixels = 16 << 20

// previewHeight is how tall a previewed picture may be. A taller one pushes
// the controls under it out of the window and brings a scroll bar with it,
// which is not what a preview is for. The picture is scaled to fit, so its
// own shape is kept whatever the room it is given.
const previewHeight = 240

// Image encodes a picture as Base64, and reads a picture back out of Base64.
type Image struct {
	win    *mygo.Window
	picks  chan pick // pictures picked to be encoded
	texts  chan pick // text picked to be read
	saves  chan saved
	busy   bool // a file dialog is up
	saving bool

	dir  int // 0 图片 → Base64, 1 Base64 → 图片
	form int // of the text made: 0 纯 Base64, 1 Data URI

	// The picture being encoded.
	path   string
	size   int
	bitmap *ui.Bitmap
	skip   string // why the picture is not shown, when it is not
	raw    string // the file as plain Base64
	uri    string // the file as a data: URI
	// short is raw and uri as the text area shows them: cut to the head of
	// each, which is all that fits and all that is worth drawing.
	short [2]string // indexed by form
	err   string

	saved   string // where the last save put the text
	saveErr string

	// The text being read, and the picture it holds.
	text      string // what was pasted or picked
	src       string // the text read last
	shownText string // the head of text, which the read-only box shows
	picData   []byte // the bytes it was read as, which the copy reads again
	pic       *ui.Bitmap
	picInfo   string // what the picture is: format, size, dimensions
	picNote   string // why it is not shown, when it is not
	picErr    string
}

func (*Image) ID() string   { return "img" }
func (*Image) Name() string { return "图片转 Base64" }
func (*Image) Desc() string { return "图片编码成 Base64 / Data URI，也能读回图片" }

// SetWindow is what WindowAware asks of a tool acting on the window.
func (im *Image) SetWindow(win *mygo.Window) { im.win = win }

// apply takes what the file dialog returned for a picture.
func (im *Image) apply(p pick) {
	im.busy = false
	switch {
	case p.err != nil:
		im.err = p.err.Error()
	case p.path == "":
		// Canceled: keep what was shown.
	default:
		im.err, im.skip, im.bitmap = "", "", nil
		im.saved, im.saveErr = "", ""
		im.path, im.size = p.path, len(p.data)
		im.raw = base64.StdEncoding.EncodeToString(p.data)
		im.uri = "data:" + http.DetectContentType(p.data) + ";base64," + im.raw
		im.short = [2]string{
			widgets.Elide(im.raw, previewChars),
			widgets.Elide(im.uri, previewChars),
		}
		im.decode(p.data)
	}
}

// applyText takes what the file dialog returned for a text file.
func (im *Image) applyText(p pick) {
	im.busy = false
	switch {
	case p.err != nil:
		im.picErr = p.err.Error()
	case p.path == "":
		// Canceled.
	default:
		im.text = string(p.data)
	}
}

// applySave takes what the save dialog returned.
func (im *Image) applySave(res saved) {
	im.saving = false
	switch {
	case res.err != nil:
		im.saved, im.saveErr = "", res.err.Error()
	case res.path == "":
		// Canceled.
	default:
		im.saved, im.saveErr = res.path, ""
	}
}

// decode prepares the preview of the picture being encoded, or says why
// there is none.
func (im *Image) decode(data []byte) {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		im.err = "无法解码图片：" + err.Error()
		return
	}
	if n := cfg.Width * cfg.Height; n > maxPreviewPixels {
		im.skip = fmt.Sprintf("%d×%d 的图片过大，这里不预览（不影响下面的 Base64）",
			cfg.Width, cfg.Height)
		return
	}
	bmp, err := ui.DecodeBitmap(data)
	if err != nil {
		im.err = "无法解码图片：" + err.Error()
		return
	}
	im.bitmap = bmp
}

// sync reads the text again when it changed, into the picture it holds.
func (im *Image) sync() {
	if im.text == im.src {
		return
	}
	im.src = im.text
	im.pic, im.picData, im.picInfo, im.picNote, im.picErr = nil, nil, "", "", ""
	text := strings.TrimSpace(im.text)
	im.shownText = widgets.Elide(text, previewChars)
	if text == "" {
		return
	}
	data, err := decodeBase64(text)
	if err != nil {
		im.picErr = err.Error()
		return
	}
	// Only the head of the file is read here, so that a picture too large
	// to draw is told of without being decoded to find that out.
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		im.picErr = "这不是一张图片：" + err.Error()
		return
	}
	im.picData = data
	im.picInfo = fmt.Sprintf("%s · %d×%d · %s",
		strings.ToUpper(format), cfg.Width, cfg.Height, humanBytes(len(data)))
	if cfg.Width*cfg.Height > maxPreviewPixels {
		im.picNote = fmt.Sprintf("%d×%d 的图片过大，不在这里预览（「复制图片」照常可用）",
			cfg.Width, cfg.Height)
		return
	}
	bmp, err := ui.DecodeBitmap(data)
	if err != nil {
		im.picErr = "这不是一张图片：" + err.Error()
		return
	}
	im.pic = bmp
}

// decodeBase64 reads what a picture is held in: the Base64 itself, or a
// data: URI around it. The line breaks of a wrapped one, and the quotes of
// one copied as a JSON string, are left out.
func decodeBase64(s string) ([]byte, error) {
	s = strings.TrimSpace(s)
	if i := strings.Index(s, "base64,"); i >= 0 {
		s = s[i+len("base64,"):]
	}
	if i := strings.IndexAny(s, "\"'"); i >= 0 {
		s = s[:i]
	}
	s = strings.Map(func(r rune) rune {
		switch r {
		case ' ', '\t', '\n', '\r':
			return -1
		}
		return r
	}, s)
	// A padded, an unpadded, and a URL-safe Base64 all turn up in the wild.
	for _, enc := range []*base64.Encoding{
		base64.StdEncoding, base64.RawStdEncoding, base64.URLEncoding,
	} {
		if data, err := enc.DecodeString(s); err == nil {
			return data, nil
		}
	}
	return nil, errors.New("不是有效的 Base64")
}

// copyPic puts the picture on the clipboard as a PNG, ready to paste into a
// chat, a document or a picture editor. It is read from the bytes the text
// held, kept whether or not the picture was small enough to draw.
func (im *Image) copyPic(c *ui.Context) {
	img, _, err := image.Decode(bytes.NewReader(im.picData))
	if err != nil {
		im.picErr = "复制失败：" + err.Error()
		return
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		im.picErr = "复制失败：" + err.Error()
		return
	}
	c.Services().WriteClipboardData(transfer.New(transfer.NewItem(
		transfer.Bytes(transfer.PNG, buf.Bytes()),
	)))
	c.Toast("已复制图片")
}

// saveName is the file name the save dialog opens with: the picture's own,
// with the form of the text it was saved as.
func (im *Image) saveName() string {
	name := filepath.Base(im.path)
	if name == "" || name == "." {
		name = "image"
	}
	if im.form == 1 {
		return name + ".datauri.txt"
	}
	return name + ".base64.txt"
}

// clear drops the picture and everything read out of it.
func (im *Image) clear() {
	im.path, im.size, im.bitmap, im.raw, im.uri, im.err = "", 0, nil, "", "", ""
	im.short, im.skip, im.saved, im.saveErr = [2]string{}, "", "", ""
}

func (im *Image) View(c *ui.Context) {
	t := c.Theme()
	if im.picks == nil {
		im.picks, im.texts, im.saves = make(chan pick, 4), make(chan pick, 4), make(chan saved, 4)
	}
	widgets.Drain(im.picks, im.apply)
	widgets.Drain(im.texts, im.applyText)
	widgets.Drain(im.saves, im.applySave)

	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		widgets.Caption(c, "方向")
		ui.Segmented(c.Key("dir"), &im.dir, "图片 → Base64", "Base64 → 图片")
	})
	if im.dir == 0 {
		im.encodeView(c, t)
	} else {
		im.decodeView(c, t)
	}
}

// encodeView is the page that turns a picture into text.
func (im *Image) encodeView(c *ui.Context, t *ui.Theme) {
	ui.Row(c).Gap(8).Wrap().AlignItems(ui.Center).Children(func() {
		pick := ui.PrimaryButton(c.Key("pick"), "选择图片…").Disabled(im.busy).
			Tooltip("选一张本地图片（" + widgets.ShortcutHint(0, "O") + "）")
		if !im.busy && (filePressed(c, pick) || mainPressed(c, pick)) {
			im.busy = true
			pickFile(im.win, c.Services(), "选择图片", imageFilters, im.picks)
		}
		if im.busy {
			widgets.Hint(c, "等待选择…")
		}
		if im.path != "" {
			widgets.Hint(c, filepath.Base(im.path)+"　"+humanBytes(im.size))
		}
		ui.Spacer(c)
		widgets.Hint(c, "支持 PNG / JPEG / GIF / WebP / BMP")
	})
	if im.err != "" {
		ui.Text(c, "错误："+im.err).FontSize(12).TextColor(t.Danger)
	}
	if im.bitmap != nil {
		ui.Column(c).PaddingY(8).Center().FillWidth().Children(func() {
			ui.Image(c.Key("preview"), im.bitmap).Label("图片预览").MaxHeight(previewHeight)
		})
	} else if im.skip != "" {
		ui.Text(c, im.skip).FontSize(12).TextColor(t.TextMuted)
	}

	// What is copied or saved is the whole text; only the showing of it is
	// cut short.
	shown, short := &im.raw, &im.short[0]
	if im.form == 1 {
		shown, short = &im.uri, &im.short[1]
	}
	ui.Row(c).Gap(8).Wrap().AlignItems(ui.Center).Children(func() {
		ui.Segmented(c.Key("form"), &im.form, "纯 Base64", "Data URI")
		ui.Spacer(c)
		widgets.CopyButton(c.Key("copy"), "复制", *shown)
		save := ui.Button(c.Key("save"), "保存为文件…").
			Tooltip("把完整的内容写进一个文本文件，自己选位置和名字（" +
				widgets.ShortcutHint(0, "S") + "）").
			Disabled(im.raw == "" || im.saving)
		if save.Clicked() || im.raw != "" && !im.saving && c.Shortcut(ui.Cmd, ui.KeyS) {
			im.saving = true
			im.saved, im.saveErr = "", ""
			saveFile(im.win, c.Services(), "保存为文本文件", im.saveName(), *shown, im.saves)
		}
		if ui.Button(c.Key("clear"), "清空").Disabled(im.raw == "").Clicked() {
			im.clear()
		}
	})
	widgets.Field(c, "结果", func() {
		widgets.Result(c, "b64", "Base64 结果", short, 6, 16)
	})
	ui.Row(c).Gap(8).Wrap().AlignItems(ui.Center).Children(func() {
		widgets.Hint(c, "图片大了这串字符会很长，这里只显示开头；「复制」和「保存为文件…」拿到的都是完整的")
		if im.saveErr != "" {
			ui.Text(c, "保存失败："+im.saveErr).FontSize(12).TextColor(t.Danger)
		} else if im.saved != "" {
			ui.Text(c, "已保存到 "+im.saved).FontSize(12).TextColor(t.Success)
		}
	})
}

// decodeView is the page that reads a picture back out of text.
func (im *Image) decodeView(c *ui.Context, t *ui.Theme) {
	im.sync()

	ui.Row(c).Gap(8).Wrap().AlignItems(ui.Center).Children(func() {
		paste := ui.PrimaryButton(c.Key("paste"), "从剪贴板粘贴").
			Tooltip("读剪贴板上的 Base64 或 data URI（" + widgets.ShortcutHint(0, "Enter") + "）")
		if mainPressed(c, paste) {
			im.text = c.Services().ReadClipboard()
		}
		pickText := ui.Button(c.Key("picktext"), "选择文件…").Disabled(im.busy).
			Tooltip("读一个装着 Base64 的文本文件（" + widgets.ShortcutHint(0, "O") + "）")
		if !im.busy && filePressed(c, pickText) {
			im.busy = true
			pickFile(im.win, c.Services(), "选择 Base64 文本文件", base64Filters, im.texts)
		}
		if ui.Button(c.Key("cleartext"), "清空").Disabled(im.text == "").Clicked() {
			im.text = ""
		}
		ui.Spacer(c)
		widgets.Hint(c, "data:image/png;base64,… 或直接的 Base64")
	})
	if im.picErr != "" {
		ui.Text(c, "错误："+im.picErr).FontSize(12).TextColor(t.Danger)
	}
	// Keyed: the line reporting an error comes and goes above it.
	widgets.FieldKey(c, "textfield", "Base64 内容", func() {
		// Read-only, and cut to its head: the text is often megabytes on a
		// single line, which takes long to lay out, and it is read from
		// the clipboard or a file rather than typed.
		ui.TextArea(c.Key("text"), &im.shownText).Label("Base64 内容").
			Placeholder("点上面的「从剪贴板粘贴」，或选一个文本文件").
			ReadOnly(true).Lines(4, 8).FillWidth()
	})

	if im.pic != nil {
		ui.Column(c).PaddingY(8).Center().FillWidth().Children(func() {
			ui.Image(c.Key("pic"), im.pic).Label("读出的图片").MaxHeight(previewHeight)
		})
	} else if im.picNote != "" {
		ui.Text(c, im.picNote).FontSize(12).TextColor(t.TextMuted)
	} else {
		ui.Column(c).PaddingY(8).Center().FillWidth().Children(func() {
			widgets.Hint(c, "读出来的图片会显示在这里")
		})
	}
	ui.Row(c).Gap(8).Wrap().AlignItems(ui.Center).Children(func() {
		copyImg := ui.PrimaryButton(c.Key("copyimg"), "复制图片").Disabled(im.picData == nil).
			Tooltip("把图片本身放进剪贴板（" + widgets.ShortcutHint(0, "Enter") + "）")
		if im.picData != nil && mainPressed(c, copyImg) {
			im.copyPic(c)
		}
		if im.picInfo != "" {
			ui.Text(c, im.picInfo).FontSize(12).TextColor(t.TextMuted)
		}
		ui.Spacer(c)
		widgets.Hint(c, "复制的是图片本身，可直接粘贴到微信、文档或图片编辑器")
	})
}
