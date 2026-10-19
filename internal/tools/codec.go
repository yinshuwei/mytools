package tools

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"

	"github.com/egoist/mygo/ui"

	"gitee.com/yinshuwei/mytools/internal/widgets"
)

// Codec encodes text to and decodes it from a transfer representation.
type Codec struct {
	mode  string // the encoding in use
	input string
	out   string
	err   string
}

func (*Codec) ID() string   { return "codec" }
func (*Codec) Name() string { return "信息编解码" }
func (*Codec) Desc() string { return "Base64、URL、Hex 的编码与解码" }

// MaxWidth keeps the boxes of text and the buttons between them in a
// column, rather than stretching them across a wide window.
func (*Codec) MaxWidth() float32 { return PageWidth }

// modes are the codecs the select offers.
var modes = []string{"Base64", "Base64 (URL)", "URL", "Hex"}

// encodeText turns s into the chosen representation.
func encodeText(mode, s string) (string, error) {
	switch mode {
	case "Base64":
		return base64.StdEncoding.EncodeToString([]byte(s)), nil
	case "Base64 (URL)":
		return base64.URLEncoding.EncodeToString([]byte(s)), nil
	case "URL":
		return url.QueryEscape(s), nil
	case "Hex":
		return hex.EncodeToString([]byte(s)), nil
	}
	return "", fmt.Errorf("未知的编码方式 %q", mode)
}

// decodeText turns the chosen representation of s back into text.
func decodeText(mode, s string) (string, error) {
	s = strings.TrimSpace(s)
	switch mode {
	case "Base64":
		b, err := base64.StdEncoding.DecodeString(s)
		return string(b), err
	case "Base64 (URL)":
		b, err := base64.URLEncoding.DecodeString(s)
		return string(b), err
	case "URL":
		return url.QueryUnescape(s)
	case "Hex":
		b, err := hex.DecodeString(strings.Join(strings.Fields(s), ""))
		return string(b), err
	}
	return "", fmt.Errorf("未知的编码方式 %q", mode)
}

// convert encodes or decodes the input into the output.
func (cd *Codec) convert(decode bool) {
	cd.err = ""
	var out string
	var err error
	if decode {
		out, err = decodeText(cd.mode, cd.input)
	} else {
		out, err = encodeText(cd.mode, cd.input)
	}
	if err != nil {
		cd.out, cd.err = "", err.Error()
		return
	}
	cd.out = out
}

func (cd *Codec) View(c *ui.Context) {
	t := c.Theme()
	if cd.mode == "" {
		cd.mode = modes[0]
	}

	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		widgets.Hint(c, "编码方式")
		ui.Select(c.Key("mode"), &cd.mode, modes).Width(160)
		ui.Spacer(c)
		widgets.Hint(c, "文本按 UTF-8 处理")
	})
	widgets.Field(c, "输入", func() {
		ui.TextArea(c.Key("in"), &cd.input).Label("文本输入").
			Placeholder("要编码或解码的文本…").Lines(5, 14).FillWidth()
	})
	ui.Row(c).Gap(8).Wrap().AlignItems(ui.Center).Children(func() {
		enc := ui.PrimaryButton(c.Key("enc"), "编码 ↓").
			Tooltip("把输入按所选方式编码（" + widgets.ShortcutHint(0, "Enter") + "）")
		if mainPressed(c, enc) {
			cd.convert(false)
		}
		dec := ui.Button(c.Key("dec"), "解码 ↓").
			Tooltip("把输入按所选方式解码（" + widgets.ShortcutHint(ui.Shift, "Enter") + "）")
		if dec.Clicked() || c.Shortcut(ui.Cmd|ui.Shift, ui.KeyEnter) {
			cd.convert(true)
		}
		if ui.Button(c.Key("swap"), "结果 → 输入").Clicked() {
			cd.input, cd.out = cd.out, ""
		}
		ui.Spacer(c)
		widgets.CopyButton(c.Key("copyin"), "复制输入", cd.input)
	})
	if cd.err != "" {
		ui.Text(c, "错误："+cd.err).FontSize(widgets.Scaled(12)).TextColor(t.Danger)
	}
	// Keyed: the line reporting an error comes and goes above it.
	widgets.FieldKey(c, "outfield", "结果", func() {
		widgets.Result(c, "out", "结果", &cd.out, 5, 14)
	})
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		widgets.CopyButton(c.Key("copyout"), "复制结果", cd.out)
		if ui.Button(c.Key("clear"), "清空").Clicked() {
			cd.input, cd.out, cd.err = "", "", ""
		}
	})
}
