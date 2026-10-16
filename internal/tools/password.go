package tools

import (
	"crypto/rand"
	"errors"
	"math/big"
	"strings"

	"github.com/egoist/mygo/ui"

	"gitee.com/yinshuwei/mytools/internal/widgets"
)

// Password makes batches of random passwords out of chosen character sets.
type Password struct {
	inited  bool
	length  float64
	count   float64
	upper   bool
	lower   bool
	digits  bool
	symbols bool
	avoid   bool // 排除易混淆字符

	text string   // the batch, one password a line
	pws  []string // and the same as the passwords they are
	err  string
}

func (*Password) ID() string   { return "pw" }
func (*Password) Name() string { return "随机密码" }
func (*Password) Desc() string { return "按字符集批量生成随机密码" }

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

// sets are the character sets the chosen ones make.
func (p *Password) sets() []string {
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
	return sets
}

// gen makes a batch of passwords.
func (p *Password) gen() {
	sets := p.sets()
	n := max(1, min(int(p.count), 200))
	out := make([]string, 0, n)
	for range n {
		pw, err := generate(int(p.length), sets, p.avoid)
		if err != nil {
			p.err = err.Error()
			return
		}
		out = append(out, pw)
	}
	p.pws, p.err = out, ""
	p.text = strings.Join(out, "\n")
}

func (p *Password) View(c *ui.Context) {
	t := c.Theme()
	if !p.inited {
		p.inited = true
		p.length, p.count = 16, 5
		p.upper, p.lower, p.digits = true, true, true
		p.gen()
	}

	ui.Row(c).Gap(10).Wrap().AlignItems(ui.Center).Children(func() {
		ui.Column(c).Gap(4).Children(func() {
			widgets.Hint(c, "长度")
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Slider(c.Key("len"), &p.length, 4, 64).Step(1).Width(180)
				ui.Textf(c, "%d", int(p.length)).Bold().Width(28)
			})
		})
		ui.Column(c).Gap(4).Children(func() {
			widgets.Hint(c, "数量")
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Slider(c.Key("count"), &p.count, 1, 50).Step(1).Width(180)
				ui.Textf(c, "%d", int(p.count)).Bold().Width(28)
			})
		})
	})
	ui.Row(c).Gap(16).Wrap().Children(func() {
		ui.Checkbox(c.Key("upper"), &p.upper, "大写 A-Z")
		ui.Checkbox(c.Key("lower"), &p.lower, "小写 a-z")
		ui.Checkbox(c.Key("digits"), &p.digits, "数字 0-9")
		ui.Checkbox(c.Key("symbols"), &p.symbols, "符号")
	})
	ui.Checkbox(c.Key("avoid"), &p.avoid, "排除易混淆字符（I l 1 O 0 o 等）")

	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		gen := ui.PrimaryButton(c.Key("gen"), "生成").
			Tooltip("再换一批（" + widgets.ShortcutHint(0, "Enter") + "）")
		if mainPressed(c, gen) {
			p.gen()
		}
		widgets.CopyButton(c.Key("copyall"), "复制全部", p.text)
		if len(p.pws) > 0 {
			ui.Textf(c, "共 %d 条", len(p.pws)).FontSize(12).TextColor(t.TextMuted)
		}
		ui.Spacer(c)
		if ui.Button(c.Key("clear"), "清空").Disabled(len(p.pws) == 0).Clicked() {
			p.pws, p.text = nil, ""
		}
	})
	if p.err != "" {
		ui.Text(c, "错误："+p.err).FontSize(12).TextColor(t.Danger)
	}

	// Keyed: the line reporting an error comes and goes above it.
	widgets.FieldKey(c, "pwsfield", "密码（每行一个）", func() {
		widgets.Result(c, "pws", "密码", &p.text, 4, 16)
	})
}
