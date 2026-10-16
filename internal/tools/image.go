package tools

import (
	"encoding/base64"
	"net/http"
	"path/filepath"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"gitee.com/yinshuwei/mytools/internal/widgets"
)

// Image encodes a picture as Base64 or a data URI.
type Image struct {
	win   *mygo.Window
	picks chan pick
	busy  bool

	path   string
	size   int
	bitmap *ui.Bitmap
	raw    string // the file as plain Base64
	uri    string // the file as a data: URI
	mode   int    // 0 plain Base64, 1 data URI
	err    string
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
		im.err = ""
		im.path, im.size = p.path, len(p.data)
		im.raw = base64.StdEncoding.EncodeToString(p.data)
		im.uri = "data:" + http.DetectContentType(p.data) + ";base64," + im.raw
		if bmp, err := ui.DecodeBitmap(p.data); err == nil {
			im.bitmap = bmp
		} else {
			im.bitmap, im.err = nil, "无法解码图片："+err.Error()
		}
	}
}

func (im *Image) View(c *ui.Context) {
	t := c.Theme()
	if im.picks == nil {
		im.picks = make(chan pick, 4)
	}
	widgets.Drain(im.picks, im.apply)

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
	}

	shown := &im.raw
	if im.mode == 1 {
		shown = &im.uri
	}
	ui.Row(c).Gap(8).Wrap().AlignItems(ui.Center).Children(func() {
		ui.Segmented(c.Key("mode"), &im.mode, "纯 Base64", "Data URI")
		ui.Spacer(c)
		widgets.CopyButton(c.Key("copy"), "复制", *shown)
		if ui.Button(c.Key("clear"), "清空").Disabled(im.raw == "").Clicked() {
			im.path, im.size, im.bitmap, im.raw, im.uri, im.err = "", 0, nil, "", "", ""
		}
	})
	widgets.Field(c, "结果", func() {
		widgets.Result(c, "b64", "Base64 结果", shown, 6, 16)
	})
	ui.Row(c).Children(func() {
		widgets.Hint(c, "大图片会产生很长的字符串，复制后即可在 HTML/CSS 中直接使用")
	})
}
