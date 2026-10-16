package tools

import (
	"bytes"
	"encoding/json"
	"strings"

	"github.com/egoist/mygo/ui"

	"gitee.com/yinshuwei/mytools/internal/widgets"
)

// JSON formats, minifies and validates JSON.
type JSON struct {
	input  string
	output string
	indent string
	err    string
}

func (*JSON) ID() string   { return "json" }
func (*JSON) Name() string { return "JSON 美化" }
func (*JSON) Desc() string { return "格式化、压缩与校验 JSON" }

// indentString is the whitespace the chosen indentation is made of.
func (j *JSON) indentString() string {
	switch j.indent {
	case "4":
		return "    "
	case "Tab":
		return "\t"
	default:
		return "  "
	}
}

// run formats the input, or minifies it.
func (j *JSON) run(minify bool) {
	j.err = ""
	src := strings.TrimSpace(j.input)
	if src == "" {
		j.output, j.err = "", "先输入 JSON"
		return
	}
	var buf bytes.Buffer
	var err error
	if minify {
		err = json.Compact(&buf, []byte(src))
	} else {
		err = json.Indent(&buf, []byte(src), "", j.indentString())
	}
	if err != nil {
		j.output, j.err = "", err.Error()
		return
	}
	j.output = buf.String()
}

func (j *JSON) View(c *ui.Context) {
	t := c.Theme()
	if j.indent == "" {
		j.indent = "2"
	}

	widgets.Field(c, "输入", func() {
		ui.TextArea(c.Key("in"), &j.input).Label("JSON 输入").Placeholder(`{"hello": "world"}`).Lines(6, 18).FillWidth()
	})
	ui.Row(c).Gap(8).Wrap().AlignItems(ui.Center).Children(func() {
		if ui.PrimaryButton(c.Key("fmt"), "格式化").Clicked() {
			j.run(false)
		}
		if ui.Button(c.Key("min"), "压缩").Clicked() {
			j.run(true)
		}
		widgets.Hint(c, "缩进")
		if ui.Select(c.Key("indent"), &j.indent, []string{"2", "4", "Tab"}).Changed() && j.output != "" {
			j.run(false)
		}
		ui.Spacer(c)
		widgets.CopyButton(c.Key("copyin"), "复制输入", j.input)
		if ui.Button(c.Key("clear"), "清空").Clicked() {
			j.input, j.output, j.err = "", "", ""
		}
	})
	if j.err != "" {
		ui.Text(c, "错误："+j.err).FontSize(12).TextColor(t.Danger)
	}
	widgets.Field(c, "结果", func() {
		widgets.Result(c, "out", "JSON 结果", &j.output, 6, 18)
	})
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		widgets.CopyButton(c.Key("copyout"), "复制结果", j.output)
		widgets.Hint(c, "结果只读，可直接选中复制")
	})
}
