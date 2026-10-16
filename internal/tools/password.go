package tools

import (
	"crypto/rand"
	"errors"
	"math/big"
	"strings"

	"github.com/egoist/mygo/ui"

	"gitee.com/yinshuwei/mytools/internal/widgets"
)

// Password makes random passwords out of chosen character sets.
type Password struct {
	inited  bool
	length  float64
	upper   bool
	lower   bool
	digits  bool
	symbols bool
	avoid   bool // 排除易混淆字符

	password string
	history  []string
	err      string
}

func (*Password) ID() string   { return "pw" }
func (*Password) Name() string { return "随机密码" }
func (*Password) Desc() string { return "按字符集生成随机密码" }

// The character sets a password is drawn from.
const (
	upperChars  = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	lowerChars  = "abcdefghijklmnopqrstuvwxyz"
	digitChars  = "0123456789"
	symbolChars = "!@#$%^&*()-_=+[]{};:,.?/~"
	// ambiguous are the characters that are easy to mix up when read.
	ambiguous = "Il1O0o|`'\"()[]{}"
)

// randInt returns a uniform random number in [0, n).
func randInt(n int) (int, error) {
	if n <= 0 {
		return 0, errors.New("空字符集")
	}
	v, err := rand.Int(rand.Reader, big.NewInt(int64(n)))
	if err != nil {
		return 0, err
	}
	return int(v.Int64()), nil
}

// generate makes a password of length characters, with at least one from
// each of the chosen sets, in random order.
func generate(length int, sets []string, avoidAmbiguous bool) (string, error) {
	keep := func(s string) string {
		if !avoidAmbiguous {
			return s
		}
		var b strings.Builder
		for _, r := range s {
			if !strings.ContainsRune(ambiguous, r) {
				b.WriteRune(r)
			}
		}
		return b.String()
	}
	var pool, required []rune
	for _, set := range sets {
		rs := []rune(keep(set))
		if len(rs) == 0 {
			continue
		}
		pool = append(pool, rs...)
		k, err := randInt(len(rs))
		if err != nil {
			return "", err
		}
		required = append(required, rs[k])
	}
	if len(pool) == 0 {
		return "", errors.New("请至少选择一类字符")
	}
	length = max(length, len(required))
	out := append(make([]rune, 0, length), required...)
	for len(out) < length {
		k, err := randInt(len(pool))
		if err != nil {
			return "", err
		}
		out = append(out, pool[k])
	}
	// Shuffle, so the characters taken from each set are not all in front.
	for i := len(out) - 1; i > 0; i-- {
		j, err := randInt(i + 1)
		if err != nil {
			return "", err
		}
		out[i], out[j] = out[j], out[i]
	}
	return string(out[:length]), nil
}

// gen makes a password and keeps it in the history.
func (p *Password) gen() {
	var sets []string
	if p.upper {
		sets = append(sets, upperChars)
	}
	if p.lower {
		sets = append(sets, lowerChars)
	}
	if p.digits {
		sets = append(sets, digitChars)
	}
	if p.symbols {
		sets = append(sets, symbolChars)
	}
	pw, err := generate(int(p.length), sets, p.avoid)
	if err != nil {
		p.err = err.Error()
		return
	}
	p.err = ""
	p.password = pw
	p.history = append([]string{pw}, p.history...)
	if len(p.history) > 10 {
		p.history = p.history[:10]
	}
}

func (p *Password) View(c *ui.Context) {
	t := c.Theme()
	if !p.inited {
		p.inited = true
		p.length, p.upper, p.lower, p.digits = 16, true, true, true
		p.gen()
	}

	ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
		widgets.Hint(c, "长度")
		ui.Slider(c.Key("len"), &p.length, 4, 64).Step(1).Width(240)
		ui.Textf(c, "%d", int(p.length)).Bold().Width(28)
	})
	ui.Row(c).Gap(16).Wrap().Children(func() {
		ui.Checkbox(c.Key("upper"), &p.upper, "大写 A-Z")
		ui.Checkbox(c.Key("lower"), &p.lower, "小写 a-z")
		ui.Checkbox(c.Key("digits"), &p.digits, "数字 0-9")
		ui.Checkbox(c.Key("symbols"), &p.symbols, "符号")
	})
	ui.Checkbox(c.Key("avoid"), &p.avoid, "排除易混淆字符（I l 1 O 0 o 等）")

	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		if ui.PrimaryButton(c.Key("gen"), "生成").Clicked() {
			p.gen()
		}
		ui.Spacer(c)
		if ui.Button(c.Key("clearh"), "清空记录").Disabled(len(p.history) == 0).Clicked() {
			p.history = nil
		}
	})
	if p.err != "" {
		ui.Text(c, "错误："+p.err).FontSize(12).TextColor(t.Danger)
	}

	widgets.Field(c, "密码", func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.TextInput(c.Key("pw"), &p.password).Label("密码").ReadOnly(true).FontSize(16).Grow(1)
			widgets.CopyButton(c.Key("copy"), "复制", p.password)
		})
	})

	if len(p.history) > 1 {
		widgets.Field(c, "最近生成", func() {
			ui.Column(c).Gap(4).FillWidth().Children(func() {
				for i, pw := range p.history[1:] {
					ui.Row(c).Key(i).Gap(8).AlignItems(ui.Center).FillWidth().Children(func() {
						ui.Text(c, pw).FontSize(13).TextColor(t.TextMuted).Grow(1).Selectable()
						widgets.CopyButton(c, "复制", pw)
					})
				}
			})
		})
	}
}
