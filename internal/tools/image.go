package tools

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"net/http"
	"path/filepath"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"gitee.com/yinshuwei/mytools/internal/widgets"
)

// previewChars is how much of the Base64 the text area shows. The whole of it
// is megabytes for a picture of any size, and laying out megabytes on every
// frame is what hangs the view; what one wants of a string that long is to
// copy it or save it, not to read it.
const previewChars = 600

// maxPreviewPixels is how large a picture may be and still be decoded for the
// preview. Past it the decode alone takes seconds and the pixels hundreds of
// megabytes, while the Base64 is what the tool is really for.
const maxPreviewPixels = 16 << 20

// Image encodes a picture as Base64 or a data URI.
type Image struct {
	win    *mygo.Window
	picks  chan pick
	saves  chan saved
	busy   bool
	saving bool

	path   string
	size   int
	bitmap *ui.Bitmap
	skip   string // why the picture is not shown, when it is not
	raw    string // the file as plain Base64
	uri    string // the file as a data: URI
	// short is raw and uri as the text area shows them: cut to the head of
	// each, which is all that fits and all that is worth drawing.
	short [2]string // indexed by mode
	mode  int       // 0 plain Base64, 1 data URI
	err   string

	saved   string // where the last save put the text
	saveErr string
}

func (*Image) ID() string   { return "img" }
func (*Image) Name() string { return "图片转 Base64" }
func (*Image) Desc() string { return "把图片编码成 Base64 或 Data URI" }

// SetWindow is what WindowAware asks of a tool acting on the window.
func (im *Image) SetWindow(win *mygo.Window) { im.win = win }

// apply takes what the file dialog returned.
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

// decode prepares the preview of the picture, or says why there is none.
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

// saveName is the file name the save dialog opens with: the picture's own,
// with the form of the text it was saved as.
func (im *Image) saveName() string {
	name := filepath.Base(im.path)
	if name == "" || name == "." {
		name = "image"
	}
	if im.mode == 1 {
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
		im.picks, im.saves = make(chan pick, 4), make(chan saved, 4)
	}
	widgets.Drain(im.picks, im.apply)
	widgets.Drain(im.saves, im.applySave)

	ui.Row(c).Gap(8).Wrap().AlignItems(ui.Center).Children(func() {
		if ui.PrimaryButton(c.Key("pick"), "选择图片…").Disabled(im.busy).Clicked() {
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
			ui.Image(c.Key("preview"), im.bitmap)
		})
	} else if im.skip != "" {
		ui.Text(c, im.skip).FontSize(12).TextColor(t.TextMuted)
	}

	// What is copied or saved is the whole text; only the showing of it is
	// cut short.
	shown, short := &im.raw, &im.short[0]
	if im.mode == 1 {
		shown, short = &im.uri, &im.short[1]
	}
	ui.Row(c).Gap(8).Wrap().AlignItems(ui.Center).Children(func() {
		ui.Segmented(c.Key("mode"), &im.mode, "纯 Base64", "Data URI")
		ui.Spacer(c)
		widgets.CopyButton(c.Key("copy"), "复制", *shown)
		if ui.Button(c.Key("save"), "保存为文件…").
			Tooltip("把完整的内容写进一个文本文件，自己选位置和名字").
			Disabled(im.raw == "" || im.saving).Clicked() {
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
