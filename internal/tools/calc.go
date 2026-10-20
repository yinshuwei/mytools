package tools

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/yinshuwei/mytools/internal/widgets"
)

// Calc works out an expression as it is typed, and keeps what it worked
// out, to look at again or to change.
type Calc struct {
	expr string // what is being typed

	src   string  // the expression worked out last
	of    string  // what the value is of: that expression, trimmed
	value string  // the value, written out
	num   float64 // the same, as a number
	// exact is the value with every digit the machine holds, when that
	// says more than the fifteen the calculator writes: 1.3*12-12 is 3.6
	// written and 3.6000000000000014 held.
	exact string
	err   string // why the expression does not work out

	hist  []calcEntry // what was worked out, the newest first
	next  int         // the id the next entry takes
	toTop bool        // the list is to scroll to its first line
	list  ui.ListState
	focus bool // the box is to take the keyboard this frame
}

// calcEntry is one line of the history.
type calcEntry struct {
	id    int
	expr  string
	value string
}

func (*Calc) ID() string   { return "calc" }
func (*Calc) Name() string { return "计算器" }
func (*Calc) Desc() string { return "输入算式即出结果，连进制一起给，历史可回填" }

// MaxWidth keeps the calculator, its history included, in a column.
func (*Calc) MaxWidth() float32 { return PageWidth }

// FillsHeight takes the whole content area: the history fills what the box
// and the result leave, and scrolls by itself.
func (*Calc) FillsHeight() bool { return true }

const (
	// calcHistory is how many entries are kept.
	calcHistory = 50
	// calcRowHeight is the height of a line of the history.
	calcRowHeight = 26
	// calcResultHeight is the room the result takes whatever it holds, so
	// that nothing under it moves as one types.
	calcResultHeight = 40
)

// sync works the expression out again when it changed.
func (cl *Calc) sync() {
	if cl.expr == cl.src {
		return
	}
	cl.src, cl.err = cl.expr, ""
	expr := strings.TrimSpace(cl.expr)
	if expr == "" {
		// Nothing being typed: what was worked out last stays on screen.
		return
	}
	v, err := calcEval(expr)
	if err != nil {
		cl.value, cl.exact, cl.of, cl.num, cl.err = "", "", "", 0, err.Error()
		return
	}
	cl.num, cl.of = v, expr
	cl.value, cl.exact = formatNumber(v)
}

// commit puts what is in the box into the history and clears the box for the
// next one; the value stays on screen, as what was just worked out.
func (cl *Calc) commit() {
	expr := strings.TrimSpace(cl.expr)
	if expr == "" || cl.err != "" {
		return
	}
	cl.next++
	cl.hist = append([]calcEntry{{id: cl.next, expr: expr, value: cl.value}}, cl.hist...)
	if len(cl.hist) > calcHistory {
		cl.hist = cl.hist[:calcHistory]
	}
	cl.of, cl.expr, cl.src = expr, "", ""
	cl.toTop, cl.focus = true, true
}

// edit puts an entry's expression back in the box, where it can be changed
// and worked out again.
func (cl *Calc) edit(i int) {
	if i < 0 || i >= len(cl.hist) {
		return
	}
	cl.expr, cl.focus = cl.hist[i].expr, true
}

// clear empties the box, the result and the reason there is none.
func (cl *Calc) clear() {
	cl.expr, cl.src, cl.of, cl.value, cl.exact, cl.err = "", "", "", "", "", ""
	cl.num, cl.focus = 0, true
}

// note is the line under the result: what it is in hex and binary, and,
// when the box is empty, which expression it came from.
func (cl *Calc) note() string {
	var parts []string
	if strings.TrimSpace(cl.expr) == "" && cl.of != "" {
		parts = append(parts, "上次 "+cl.of)
	}
	if b := calcBases(cl.num); b != "" {
		parts = append(parts, b)
	}
	return strings.Join(parts, "　")
}

func (cl *Calc) View(c *ui.Context) {
	t := c.Theme()
	cl.sync()
	// Escape clears what is being typed, as a calculator's clear key
	// does; what was worked out last stays, so that a slip costs only
	// the expression being typed and not the result.
	if cl.expr != "" && c.Shortcut(0, ui.KeyEscape) {
		cl.expr = ""
	}

	ui.Column(c).FillWidth().FillHeight().Gap(12).Children(func() {
		ui.Column(c).FillWidth().Shrink(0).Gap(8).Children(func() {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				box := ui.TextInput(c.Key("expr"), &cl.expr).Label("表达式").
					Placeholder("例如 (1+2)*3、0x2f、1_000、sqrt(2)").
					Tooltip("边打边算；Enter 记入历史，Esc 清空").
					AutoFocus().Grow(1)
				if cl.focus {
					// Only the frame it was asked for: a click decides
					// where the keyboard is from then on.
					cl.focus = false
					box.Focus()
				}
				if box.Submitted() {
					cl.commit()
				}
				copyResult := ui.PrimaryButton(c.Key("copy"), "复制结果").
					Disabled(cl.value == "").
					Tooltip("把结果放进剪贴板（" + widgets.ShortcutHint(0, "Enter") + "）")
				if cl.value != "" && mainPressed(c, copyResult) {
					c.WriteClipboard(cl.value)
					c.Toast("已复制结果")
				}
				commit := ui.Button(c.Key("commit"), "记入历史").
					Disabled(cl.expr == "" || cl.err != "").
					Tooltip("把这条算式和结果记到下面，输入框清空（Enter）")
				if commit.Clicked() {
					cl.commit()
				}
				if ui.Button(c.Key("clear"), "清空").Disabled(cl.expr == "" && cl.value == "").
					Tooltip("清空输入框和结果（Esc 只清输入框）").Clicked() {
					cl.clear()
				}
			})
			ui.Row(c).Gap(12).AlignItems(ui.Center).MinHeight(calcResultHeight).Children(func() {
				switch {
				case cl.err != "":
					ui.Text(c, "错误："+cl.err).FontSize(widgets.Scaled(12)).TextColor(t.Danger)
				case cl.value != "":
					// What is written is a calculator's digits; what the
					// machine holds is on hovering, where it is wanted.
					result := ui.Text(c, cl.value).FontSize(widgets.Scaled(26)).Bold().Selectable()
					if cl.exact != "" {
						result.Tooltip("浮点数里其实是 " + cl.exact + "，这里按 15 位有效数字显示")
					}
					if note := cl.note(); note != "" {
						widgets.Hint(c, note)
					}
				default:
					widgets.Hint(c, "输入算式后这里出结果")
				}
				ui.Spacer(c)
				widgets.Hint(c, "Enter 记入历史 · Esc 清空输入")
			})
		})
		ui.Column(c).Grow(1).FillWidth().MinWidth(0).Gap(6).Children(func() {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				widgets.Caption(c, "历史")
				if n := len(cl.hist); n > 0 {
					widgets.Hint(c, fmt.Sprintf("%d 条", n))
				}
				ui.Spacer(c)
				if ui.Button(c.Key("clearhist"), "清空历史").Disabled(len(cl.hist) == 0).
					Tooltip("把历史都删掉").Clicked() {
					cl.hist = nil
				}
			})
			if len(cl.hist) == 0 {
				ui.Column(c).Grow(1).Center().Children(func() {
					widgets.Hint(c, "算过的留在这里，点一条可以放回输入框改")
				})
			} else {
				cl.list.Key = func(i int) any { return cl.hist[i].id }
				if cl.toTop {
					cl.list.ScrollIntoView(0)
					cl.toTop = false
				}
				ui.List(c, &cl.list, len(cl.hist), func(i int) { cl.row(c, i) }).
					Label("历史记录").FillWidth().Grow(1)
			}
		})
	})
}

// row is one line of the history: what was worked out, a button copying its
// value, and, on a click anywhere else, the expression going back into the
// box to be changed.
func (cl *Calc) row(c *ui.Context, i int) {
	if i < 0 || i >= len(cl.hist) {
		return
	}
	e := cl.hist[i]
	t := c.Theme()
	box := ui.Row(c).Height(calcRowHeight).FillWidth().AlignItems(ui.Center).Gap(6).PaddingX(4).
		Tooltip("点一下把 " + e.expr + " 放回输入框改")
	if box.Hovered() {
		box.Background(t.Surface)
	}
	var copyValue ui.Element
	box.Children(func() {
		ui.Text(c, e.expr).TextColor(t.TextMuted).SingleLine()
		ui.Text(c, "=").TextColor(t.TextMuted).Shrink(0)
		ui.Text(c, e.value).SingleLine().Shrink(0).Tooltip("点「复制」把它放进剪贴板")
		ui.Spacer(c)
		copyValue = ui.Button(c.Key("copy"), "复制").FontSize(widgets.Scaled(11)).Padding(0, 6).Shrink(0).
			Tooltip("复制结果 " + e.value)
		if copyValue.Clicked() {
			c.WriteClipboard(e.value)
			c.Toast("已复制结果")
		}
	})
	if box.Clicked() && !copyValue.Hovered() {
		cl.edit(i)
	}
}
