package tools

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"math/rand"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/transfer"
	"github.com/egoist/mygo/ui"

	"gitee.com/yinshuwei/mytools/internal/widgets"
)

// TestJSONTree checks the parse, the output and the folding.
func TestJSONTree(t *testing.T) {
	const src = `{"b":1,"a":[true,null,"x"]}`
	j := &JSON{input: src, indent: "2"}
	j.sync()
	if j.err != "" {
		t.Fatal(j.err)
	}
	// Members keep the order they were written in.
	want := "{\n  \"b\": 1,\n  \"a\": [\n    true,\n    null,\n    \"x\"\n  ]\n}"
	if got := j.block(j.root); got != want {
		t.Errorf("格式化 =\n%s\nwant\n%s", got, want)
	}
	if got := j.compact(); got != src {
		t.Errorf("压缩 = %q, want %q", got, src)
	}
	// What is written must read back as the same document.
	if again, err := parseJSON(j.block(j.root)); err != nil || again == nil {
		t.Errorf("输出不是有效的 JSON：%v", err)
	}
	if j.nodes != 6 {
		t.Errorf("值数 = %d, want 6", j.nodes)
	}
	// Six values, and a line closing each of the two containers.
	if len(j.flat) != 8 {
		t.Fatalf("行数 = %d, want 8", len(j.flat))
	}
	if last := j.flat[len(j.flat)-1]; !last.close || last.node.kind != '{' {
		t.Errorf("末行应当是根对象的 %s", last.node.closeBrace())
	}
	if closing := j.flat[6]; !closing.close || closing.node.closeBrace() != "]" {
		t.Errorf("数组的收尾行应当是 ]")
	}
	// A block copies as the value on its own.
	if got := j.block(j.flat[1].node); got != "1" {
		t.Errorf("块 = %q, want 1", got)
	}
	// Folding the array hides what is inside it, its brace too.
	j.toggle(j.flat[2].node)
	if len(j.flat) != 4 {
		t.Errorf("折叠数组后行数 = %d, want 4", len(j.flat))
	}
	// Folding all keeps the top level in view.
	j.foldAll()
	if len(j.flat) != 4 {
		t.Errorf("全部折叠后行数 = %d, want 4", len(j.flat))
	}
	j.toggle(j.flat[0].node)
	if len(j.flat) != 1 {
		t.Errorf("折叠根后行数 = %d, want 1", len(j.flat))
	}
	j.unfoldAll()
	if len(j.flat) != 8 {
		t.Errorf("展开后行数 = %d, want 8", len(j.flat))
	}
}

// TestJSONErrors checks what is refused.
func TestJSONErrors(t *testing.T) {
	for _, in := range []string{"{oops", `{"a":1} {"b":2}`, "[1,]", ""} {
		j := &JSON{input: in}
		j.sync()
		if in == "" {
			if j.err != "" || j.root != nil {
				t.Errorf("空输入不该报错，得到 %q", j.err)
			}
			continue
		}
		if j.err == "" || j.root != nil {
			t.Errorf("%q 应当报错，得到 err=%q", in, j.err)
		}
	}
}

// TestJSONRaw checks what copying a string's name and content gives.
func TestJSONRaw(t *testing.T) {
	j := &JSON{input: `{"greeting":"hi\nthere","n":1}`}
	j.sync()
	if j.err != "" {
		t.Fatal(j.err)
	}
	member := j.flat[1].node
	if member.key != "greeting" {
		t.Errorf("名字 = %q, want greeting", member.key)
	}
	if member.raw != "hi\nthere" {
		t.Errorf("字符串内容 = %q, want %q", member.raw, "hi\nthere")
	}
	if member.text != `"hi\nthere"` {
		t.Errorf("块 = %q, want %q", member.text, `"hi\nthere"`)
	}
	// A number has no raw content of its own.
	if n := j.flat[2].node; n.raw != "" {
		t.Errorf("数字不该有字符串内容，得到 %q", n.raw)
	}
}

// TestMatch checks what looking for a pattern finds.
func TestMatch(t *testing.T) {
	for _, tc := range []struct {
		pattern, s string
		want       bool
	}{
		{"aa", "xxaazz", true},
		{"aa", "xxzz", false},
		{"aa*bb", "aa xx bb", true},
		{"aa*bb", "bb aa", false},
		{"*aa", "xxaa", true},
		{"aa*", "aaBB", true},
		{"", "anything", true},
		{"a*b*c", "a1b2c", true},
		{"a*b*c", "a1c2b", false},
		{"*", "anything", true},
	} {
		if got := match(tc.pattern, tc.s, true); got != tc.want {
			t.Errorf("match(%q, %q) = %v, want %v", tc.pattern, tc.s, got, tc.want)
		}
	}
	if !match("AA", "xxaazz", false) {
		t.Error("不区分大小写时 AA 应当找到 xxaazz")
	}
	if match("AA", "xxaazz", true) {
		t.Error("区分大小写时 AA 不该找到 xxaazz")
	}
}

// currentRow is the line showing the current match, -1 when none does.
func currentRow(j *JSON) int {
	for i, r := range j.flat {
		if r.current {
			return i
		}
	}
	return -1
}

// TestJSONFind checks that looking finds every match in order, and that
// going to one shows it, without taking anything out of the tree.
func TestJSONFind(t *testing.T) {
	j := &JSON{input: `{"user":{"name":"Ada","city":"Paris"},"count":3,"other":{"name":"Bob"}}`}
	j.sync()
	if j.err != "" {
		t.Fatal(j.err)
	}
	all := len(j.flat)

	j.query = "name"
	j.sync()
	if len(j.found) != 2 {
		t.Fatalf("匹配数 = %d, want 2", len(j.found))
	}
	// Looking takes nothing out of the tree.
	if len(j.flat) != all {
		t.Errorf("查找后行数 = %d, want %d（不过滤）", len(j.flat), all)
	}
	lines := 0
	for _, r := range j.flat {
		if r.match {
			lines++
		}
	}
	if lines != 2 {
		t.Errorf("高亮的行 = %d, want 2", lines)
	}
	// A new query looks at its first match, which shows.
	if j.at != 0 {
		t.Errorf("当前匹配 = %d, want 0", j.at)
	}
	if currentRow(j) < 0 {
		t.Error("当前匹配的行应当显示出来")
	}
	if got := j.count(); got != "1/2" {
		t.Errorf("计数 = %q, want 1/2", got)
	}
	// Going on wraps round, both ways.
	j.step(1)
	if j.at != 1 {
		t.Errorf("下一个 = %d, want 1", j.at)
	}
	if got := j.count(); got != "2/2" {
		t.Errorf("计数 = %q, want 2/2", got)
	}
	j.step(1)
	if j.at != 0 {
		t.Errorf("再下一个应当回到开头，得到 %d", j.at)
	}
	j.step(-1)
	if j.at != 1 {
		t.Errorf("上一个应当回到末尾，得到 %d", j.at)
	}
	// Going to a match opens what holds it, and says where to scroll.
	j.foldAll()
	j.step(1)
	row := currentRow(j)
	if row < 0 {
		t.Fatal("折叠起来后走到匹配，应当把它展开")
	}
	if row != j.toShow {
		t.Errorf("应当滚动到当前匹配：行 %d，滚动到 %d", row, j.toShow)
	}
	// A value is found by its text.
	j.query = "Bob"
	j.sync()
	if len(j.found) != 1 || j.at != 0 {
		t.Errorf("按值查找 = %d 个匹配，当前 %d", len(j.found), j.at)
	}
	// Nothing found leaves the tree alone, and goes nowhere.
	j.query = "nowhere*at*all"
	j.sync()
	if len(j.found) != 0 || j.at != -1 {
		t.Errorf("无匹配时 = %d 个匹配，当前 %d", len(j.found), j.at)
	}
	if got := j.count(); got != "无匹配" {
		t.Errorf("计数 = %q, want 无匹配", got)
	}
	j.step(1)
	if j.at != -1 {
		t.Errorf("没有匹配时走不动，当前 %d", j.at)
	}
	if len(j.flat) != all {
		t.Errorf("无匹配时树应当照旧，行数 = %d, want %d", len(j.flat), all)
	}
	// Clearing the query drops the matches.
	j.query = ""
	j.sync()
	if len(j.found) != 0 || j.at != -1 || currentRow(j) >= 0 {
		t.Errorf("清空查找后还有 %d 个匹配，当前 %d", len(j.found), j.at)
	}
}

// TestJSONClickCopy checks what a click on a line of the tree copies: the
// name it lands on, the value's content, or the whole block.
func TestJSONClickCopy(t *testing.T) {
	j := &JSON{input: `{"name":"Ada","n":7,"tags":["a"]}`}
	tr := ui.NewTester(func(c *ui.Context) { j.View(c) }, 700, 500)
	block := "[\n  \"a\"\n]"
	for _, tc := range []struct {
		find string // what to click, by the text it shows
		want string
	}{
		{`"name":`, "name"},
		{`"Ada"`, "Ada"},
		{`"n":`, "n"},
		{"7", "7"},
		{`"tags":`, "tags"},
		{"[", block},
		{"]", block},
		{`"a"`, "a"},
	} {
		if err := tr.Click(tc.find); err != nil {
			t.Fatalf("找不到 %q：%v", tc.find, err)
		}
		if got := tr.Clipboard(); got != tc.want {
			t.Errorf("点击 %q 复制到 %q, want %q", tc.find, got, tc.want)
		}
	}
	// Anywhere else on a line copies its block.
	r, ok := tr.Find(`"Ada"`)
	if !ok {
		t.Fatal("找不到值")
	}
	tr.ClickAt(r.X+r.W+24, r.Y+r.H/2)
	if got, want := tr.Clipboard(), `"Ada"`; got != want {
		t.Errorf("点击行的空白处复制到 %q, want %q", got, want)
	}
	// The fold arrow folds, and copies nothing.
	if err := tr.Click("▾"); err != nil {
		t.Fatal(err)
	}
	if len(j.flat) != 1 {
		t.Errorf("点击箭头后行数 = %d, want 1", len(j.flat))
	}
	if got := tr.Clipboard(); got != `"Ada"` {
		t.Errorf("点击箭头不该复制，剪贴板变成 %q", got)
	}
}

// TestJSONFindKeys checks the keys that move between matches: Enter or
// Down goes to the next one, Shift+Enter or Up to the one before.
func TestJSONFindKeys(t *testing.T) {
	j := &JSON{input: `{"a":{"name":"one"},"b":{"name":"two"},"c":{"name":"three"}}`}
	tr := ui.NewTester(func(c *ui.Context) { j.View(c) }, 700, 500)
	if err := tr.Click("查找"); err != nil {
		t.Fatal(err)
	}
	tr.Type("name")
	if len(j.found) != 3 {
		t.Fatalf("匹配数 = %d, want 3", len(j.found))
	}
	for _, tc := range []struct {
		mods ui.Modifiers
		key  ui.Key
		want int
	}{
		{0, ui.KeyEnter, 1},
		{ui.Shift, ui.KeyEnter, 0},
		{0, ui.KeyDown, 1},
		{0, ui.KeyDown, 2},
		{ui.Shift, ui.KeyEnter, 1},
		{0, ui.KeyUp, 0},
		{ui.Shift, ui.KeyEnter, 2}, // wraps back to the last
	} {
		tr.Key(tc.mods, tc.key)
		if j.at != tc.want {
			t.Errorf("%v+%v 后当前 = %d, want %d", tc.mods, tc.key, j.at, tc.want)
		}
	}
	// The match gone to shows, wherever it is (the frames the tester ran
	// took the scroll it asked for).
	if currentRow(j) < 0 {
		t.Error("当前匹配的行应当显示出来")
	}
}

// tallBitmap is a picture far taller than any preview should be.
func tallBitmap() *ui.Bitmap {
	img := image.NewRGBA(image.Rect(0, 0, 300, 1200))
	for i := range img.Pix {
		img.Pix[i] = 0xff
	}
	return ui.NewBitmap(img)
}

// TestImagePreviewHeight checks that a tall picture is previewed short
// enough that the page under it still shows in one window, both ways round.
func TestImagePreviewHeight(t *testing.T) {
	const w, h = 900, 800
	encode := &Image{bitmap: tallBitmap(), path: "/pics/tall.png", size: 4096}
	tr := ui.NewTester(func(c *ui.Context) { encode.View(c) }, w, h)
	preview, ok := tr.Find("图片预览")
	if !ok {
		t.Fatal("找不到预览")
	}
	if preview.H > previewHeight {
		t.Errorf("预览高 %.0f, want ≤ %d", preview.H, previewHeight)
	}
	// What comes under the preview is still in the window: otherwise the
	// page scrolls, which is what the cap is for.
	if last, ok := tr.Find("Base64 结果"); !ok {
		t.Fatal("找不到结果框")
	} else if bottom := last.Y + last.H + 40; bottom > h {
		t.Errorf("页面高到 %.0f，窗口只有 %d，会出滚动条", bottom, h)
	}

	decode := &Image{dir: 1, text: "data:image/png;base64,AAAA"}
	decode.sync()
	decode.pic = tallBitmap()
	tr = ui.NewTester(func(c *ui.Context) { decode.View(c) }, w, h)
	pic, ok := tr.Find("读出的图片")
	if !ok {
		t.Fatal("找不到读出的图片")
	}
	if pic.H > previewHeight {
		t.Errorf("读出的图片高 %.0f, want ≤ %d", pic.H, previewHeight)
	}
	if last, ok := tr.Find("复制图片"); !ok {
		t.Fatal("找不到复制图片")
	} else if bottom := last.Y + last.H + 40; bottom > h {
		t.Errorf("页面高到 %.0f，窗口只有 %d，会出滚动条", bottom, h)
	}
}

// TestJSONFindIsRightAligned checks that the search and what belongs to it
// end at the right edge of the pane, with the folds left behind on the other
// side of the line.
func TestJSONFindIsRightAligned(t *testing.T) {
	j := &JSON{input: `{"a":1}`}
	tr := ui.NewTester(func(c *ui.Context) { j.View(c) }, 1200, 700)
	find, ok := tr.Find("查找")
	if !ok {
		t.Fatal("找不到查找框")
	}
	// The case switch is the last of the search's parts.
	last, ok := tr.Find("Aa")
	if !ok {
		t.Fatal("找不到区分大小写")
	}
	tree, ok := tr.Find("JSON 树")
	if !ok {
		t.Fatal("找不到结果树")
	}
	if edge, gap := tree.X+tree.W, tree.X+tree.W-(last.X+last.W); gap > 2 {
		t.Errorf("查找那一组到 pane 右边缘还有 %.0f，没有右对齐（边缘 %.0f，右端 %.0f）",
			gap, edge, last.X+last.W)
	}
	fold, ok := tr.Find("折叠")
	if !ok {
		t.Fatal("找不到折叠按钮")
	}
	if fold.X+fold.W >= find.X {
		t.Errorf("折叠按钮（右端 %.0f）应当在查找框（左端 %.0f）左边", fold.X+fold.W, find.X)
	}
}

// TestJSONFindShortcuts checks that Cmd+F takes the keyboard to the search
// from wherever it was, and that Escape clears what is looked for without
// touching what is written.
func TestJSONFindShortcuts(t *testing.T) {
	const src = `{"a":{"name":"one"},"b":{"name":"two"}}`
	j := &JSON{input: src}
	tr := ui.NewTester(func(c *ui.Context) { j.View(c) }, 900, 600)
	if err := tr.Click("JSON 输入"); err != nil {
		t.Fatal(err)
	}
	tr.Key(ui.Cmd, ui.KeyF)
	if !tr.Focused("查找") {
		t.Fatal("Cmd+F 没有把键盘给查找框")
	}
	tr.Type("name")
	if j.query != "name" || len(j.found) != 2 {
		t.Fatalf("查找 %q 找到 %d 个，want name 2 个", j.query, len(j.found))
	}
	// From the text view too: the tree is what there is to search.
	if err := tr.Click("文本"); err != nil {
		t.Fatal(err)
	}
	tr.Key(ui.Cmd, ui.KeyF)
	if j.mode != 0 || !tr.Focused("查找") {
		t.Errorf("文本视图里 Cmd+F 后 mode=%d，聚焦查找框 %v", j.mode, tr.Focused("查找"))
	}
	// Escape clears what is looked for, wherever the keyboard is.
	if err := tr.Click("JSON 输入"); err != nil {
		t.Fatal(err)
	}
	tr.Key(0, ui.KeyEscape)
	if j.query != "" || len(j.found) != 0 {
		t.Errorf("Esc 之后查找 = %q，匹配 %d 个", j.query, len(j.found))
	}
	if j.input != src {
		t.Errorf("Esc 动了输入：%q", j.input)
	}
}

// TestJSONInputKeepsEditing checks that the input keeps its place when the
// text goes invalid: the line reporting the error comes and goes, which
// must not remake the input, putting the caret back at its end and dropping
// what undoing it would give back.
func TestJSONInputKeepsEditing(t *testing.T) {
	j := &JSON{}
	tr := ui.NewTester(func(c *ui.Context) { j.View(c) }, 900, 600)
	if err := tr.Click("JSON 输入"); err != nil {
		t.Fatal(err)
	}
	tr.Type(`{"a": 1}`)
	tr.Key(0, ui.KeyLeft)
	tr.Key(0, ui.KeyLeft)
	tr.Type(",") // before the 1: the text is no longer JSON
	if j.err == "" {
		t.Fatal("不合法了，应当报错")
	}
	tr.Type("x")
	if got, want := j.input, `{"a": ,x1}`; got != want {
		t.Errorf("输入 = %q, want %q（回车符应当还在原处）", got, want)
	}
}

// TestJSONInputUndo checks that undoing still works after the text went
// from invalid to valid, which must not drop the input's history.
func TestJSONInputUndo(t *testing.T) {
	j := &JSON{}
	tr := ui.NewTester(func(c *ui.Context) { j.View(c) }, 900, 600)
	if err := tr.Click("JSON 输入"); err != nil {
		t.Fatal(err)
	}
	// Typing it goes invalid at once — nothing is JSON yet — and valid at
	// the last character, so the line reporting the error comes and goes.
	tr.Type(`{"a": 1}`)
	if j.err != "" {
		t.Fatalf("应当是合法的，得到 %q", j.err)
	}
	tr.Key(ui.Cmd, ui.KeyZ)
	if j.input == `{"a": 1}` {
		t.Errorf("撤销没有生效，输入还是 %q", j.input)
	}
}

// TestQuoteJSON checks the quoting of strings.
func TestQuoteJSON(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{`a"b`, `"a\"b"`},
		{"a\nb", `"a\nb"`},
		{"中文", `"中文"`},
		{"\x01", `"\u0001"`},
		{`c:\x`, `"c:\\x"`},
	} {
		if got := quoteJSON(tc.in); got != tc.want {
			t.Errorf("quoteJSON(%q) = %s, want %s", tc.in, got, tc.want)
		}
	}
}

// TestCodec round trips every codec.
func TestCodec(t *testing.T) {
	const text = "你好, world? a=b&c=1 ✓"
	for _, mode := range modes {
		enc, err := encodeText(mode, text)
		if err != nil {
			t.Fatalf("%s：编码失败 %v", mode, err)
		}
		got, err := decodeText(mode, enc)
		if err != nil {
			t.Fatalf("%s：解码失败 %v", mode, err)
		}
		if got != text {
			t.Errorf("%s：往返 = %q, want %q", mode, got, text)
		}
	}
	if _, err := decodeText("Hex", "zz"); err == nil {
		t.Error("Hex 解码无效输入应当报错")
	}
}

// TestCodecShortcuts checks that the keyboard presses the page's buttons
// while one is typing in it: Cmd+Enter the main one, encoding, and
// Cmd+Shift+Enter the other, decoding.
func TestCodecShortcuts(t *testing.T) {
	cd := &Codec{mode: "Base64", input: "你好"}
	tr := ui.NewTester(func(c *ui.Context) { cd.View(c) }, 900, 700)
	if err := tr.Click("文本输入"); err != nil {
		t.Fatal(err)
	}
	tr.Type("!")
	if !strings.Contains(cd.input, "!") {
		t.Fatalf("没有点到输入框，输入 = %q", cd.input)
	}
	text := cd.input
	tr.Key(ui.Cmd, ui.KeyEnter)
	if want := base64.StdEncoding.EncodeToString([]byte(text)); cd.out != want {
		t.Fatalf("Cmd+Enter 后 = %q, want %q", cd.out, want)
	}
	// The other way round: what was encoded goes back in, and decodes.
	if err := tr.Click("结果 → 输入"); err != nil {
		t.Fatal(err)
	}
	if cd.input != base64.StdEncoding.EncodeToString([]byte(text)) {
		t.Fatalf("回填后输入 = %q", cd.input)
	}
	tr.Key(ui.Cmd|ui.Shift, ui.KeyEnter)
	if cd.err != "" || cd.out != text {
		t.Errorf("Cmd+Shift+Enter 后 = %q (err=%q), want %q", cd.out, cd.err, text)
	}
}

// TestPasswordShortcut checks that Cmd+Enter makes another batch.
func TestPasswordShortcut(t *testing.T) {
	p := &Password{inited: true, length: 12, count: 3, upper: true, lower: true, digits: true}
	p.gen()
	before := p.text
	tr := ui.NewTester(func(c *ui.Context) { p.View(c) }, 900, 700)
	tr.Key(ui.Cmd, ui.KeyEnter)
	if p.text == before {
		t.Error("Cmd+Enter 没有换一批")
	}
	if len(p.pws) != 3 {
		t.Errorf("批 = %d 条, want 3", len(p.pws))
	}
}

// TestCalcEval checks what an expression works out to, and what is refused.
func TestCalcEval(t *testing.T) {
	for _, tc := range []struct {
		expr string
		want float64
	}{
		{"1+2*3", 7},
		{"(1+2)*3", 9},
		{"10-2-3", 5},
		{"100/4/5", 5},
		{"10 % 3", 1},
		{"2^10", 1024},
		{"2^3^2", 512},
		{"2^-1", 0.5},
		{"-2^2", -4},
		{"+3", 3},
		{"1.5*2", 3},
		{".5+.25", 0.75},
		{"1e3", 1000},
		{"1_000_000", 1e6},
		{"0xff", 255},
		{"0b1010", 10},
		{"0o17", 15},
		{"0x_ff_ff", 65535},
		{"abs(-3)", 3},
		{"min(3,1,2)", 1},
		{"max(1,2,3)", 3},
		{"hypot(3,4)", 5},
		{"sqrt(16)", 4},
		{"log(100)", 2},
		{"ln(e)", 1},
		{"round(2.5)", 3},
		{"floor(-1.5)", -2},
		{"pi", math.Pi},
		{"2*tau", 4 * math.Pi},
	} {
		got, err := calcEval(tc.expr)
		if err != nil {
			t.Errorf("%s：%v", tc.expr, err)
			continue
		}
		if math.Abs(got-tc.want) > 1e-9*math.Max(1, math.Abs(tc.want)) {
			t.Errorf("%s = %v, want %v", tc.expr, got, tc.want)
		}
	}
	for _, expr := range []string{
		"", "1+", "(1+2", "1+2)", "1 2", "1/0", "10%0", "0x", "1e", "foo", "foo(1)",
		"sqrt(1,2)", "min(1)", "1..2", "*3",
	} {
		if v, err := calcEval(expr); err == nil {
			t.Errorf("%q 应当报错，得到 %v", expr, v)
		}
	}
}

// TestCalcBases checks the hex and binary forms given under a result.
func TestCalcBases(t *testing.T) {
	for _, tc := range []struct {
		v    float64
		want string
	}{
		{0, "0x0 · 0b0"},
		{162, "0xa2 · 0b10100010"},
		{255, "0xff · 0b11111111"},
		{-10, "0x-a · 0b-1010"},
		{1.5, ""},
		{1e16, ""},
	} {
		if got := calcBases(tc.v); got != tc.want {
			t.Errorf("calcBases(%v) = %q, want %q", tc.v, got, tc.want)
		}
	}
}

// TestCalcPage checks the page: what is typed is worked out as it is typed,
// Enter puts it in the history, and Esc clears the box without losing what
// was worked out.
func TestCalcPage(t *testing.T) {
	cl := &Calc{}
	tr := ui.NewTester(func(c *ui.Context) { cl.View(c) }, 900, 700)
	// The box takes the keyboard as the page opens: typing goes into it.
	tr.Type("1+2*3")
	if cl.expr != "1+2*3" || cl.value != "7" {
		t.Fatalf("输入 %q，结果 %q", cl.expr, cl.value)
	}
	if !tr.HasText("0x7") {
		t.Errorf("结果里没有十六进制：%q", tr.Texts())
	}
	// Enter puts it in the history, and clears the box for the next one.
	tr.Key(0, ui.KeyEnter)
	if len(cl.hist) != 1 || cl.hist[0].expr != "1+2*3" || cl.hist[0].value != "7" {
		t.Fatalf("历史 = %+v", cl.hist)
	}
	if cl.value != "7" || !tr.HasText("上次 1+2*3") {
		t.Errorf("记入历史后结果应当还留着：value=%q", cl.value)
	}
	tr.Type("4*5")
	if cl.expr != "4*5" || cl.value != "20" {
		t.Fatalf("清空后接着打 = %q（结果 %q）", cl.expr, cl.value)
	}
	// Cmd+Enter copies the result.
	tr.Key(ui.Cmd, ui.KeyEnter)
	if got := tr.Clipboard(); got != "20" {
		t.Errorf("剪贴板 = %q, want 20", got)
	}
	// Esc clears what is typed; what was worked out stays.
	tr.Key(0, ui.KeyEscape)
	if cl.expr != "" || cl.value != "20" {
		t.Errorf("Esc 后输入 = %q，结果 = %q", cl.expr, cl.value)
	}
}

// TestCalcHistory checks that a line of the history puts its expression back
// in the box on a click, and hands over its result on its own button.
func TestCalcHistory(t *testing.T) {
	cl := &Calc{hist: []calcEntry{{id: 1, expr: "(1+2)*3", value: "9"}}}
	tr := ui.NewTester(func(c *ui.Context) { cl.View(c) }, 900, 700)
	if err := tr.Click("复制"); err != nil {
		t.Fatal(err)
	}
	if got := tr.Clipboard(); got != "9" {
		t.Errorf("剪贴板 = %q, want 9", got)
	}
	if cl.expr != "" {
		t.Errorf("点复制按钮不该把算式放回输入框，得到 %q", cl.expr)
	}
	if err := tr.Click("(1+2)*3"); err != nil {
		t.Fatal(err)
	}
	if cl.expr != "(1+2)*3" {
		t.Fatalf("点历史后输入 = %q, want (1+2)*3", cl.expr)
	}
	// What comes back is worked out again, ready to be changed.
	if cl.value != "9" {
		t.Errorf("回填后结果 = %q, want 9", cl.value)
	}
}

// TestClock checks both directions of the conversion.
func TestClock(t *testing.T) {
	for _, tc := range []struct {
		unit string
		ts   string
	}{
		{"秒", "1000000000"},
		{"毫秒", "1000000000000"},
	} {
		cl := &Clock{unit: tc.unit, ts: tc.ts}
		tt, err := cl.fromTimestamp()
		if err != nil || tt.Unix() != 1000000000 {
			t.Errorf("%s：%v %v", tc.unit, tt, err)
		}
	}
	for _, in := range []string{"2006-01-02 15:04:05", "2006-01-02T15:04:05", "2006-01-02"} {
		if _, err := parseTime(in, time.Local); err != nil {
			t.Errorf("parseTime(%q) = %v", in, err)
		}
	}
	if _, err := parseTime("不是时间", time.Local); err == nil {
		t.Error("parseTime 应当报错")
	}
}

// TestClockZone checks that one instant reads differently in another zone,
// and that a time typed in that zone is the same instant.
func TestClockZone(t *testing.T) {
	cl := &Clock{unit: "秒", tz: "UTC", ts: "1000000000"}
	instant, err := cl.fromTimestamp()
	if err != nil {
		t.Fatal(err)
	}
	loc, err := cl.location()
	if err != nil {
		t.Fatal(err)
	}
	if got := instant.In(loc).Format("2006-01-02 15:04:05"); got != "2001-09-09 01:46:40" {
		t.Errorf("UTC 下 = %q, want 2001-09-09 01:46:40", got)
	}
	cl.tz = "Asia/Shanghai"
	if loc, err = cl.location(); err != nil {
		t.Fatal(err)
	}
	if got := instant.In(loc).Format("2006-01-02 15:04:05"); got != "2001-09-09 09:46:40" {
		t.Errorf("上海下 = %q, want 2001-09-09 09:46:40", got)
	}
	// Reading a time in a zone back gives the same instant.
	tt, err := parseTime("2001-09-09 09:46:40", loc)
	if err != nil {
		t.Fatal(err)
	}
	if tt.Unix() != 1000000000 {
		t.Errorf("解析得到 %d, want 1000000000", tt.Unix())
	}
	// A zone typed in that is not known is told of, and the system's own
	// zone is what everything falls back to.
	cl = &Clock{tz: customZone, custom: "Not/AZone"}
	if loc, err := cl.location(); err == nil || loc != time.Local {
		t.Errorf("未知时区应当报错并退回本地，得到 %v %v", loc, err)
	}
	for _, cl := range []*Clock{{}, {tz: localZone}, {tz: customZone}} {
		loc, err := cl.location()
		if err != nil || loc != time.Local {
			t.Errorf("默认应当是本地时区，得到 %v %v", loc, err)
		}
	}
}

// TestPassword checks the length and that every chosen set shows.
func TestPassword(t *testing.T) {
	sets := []string{upperChars, lowerChars, digitChars, symbolChars}
	pw, err := generate(24, sets, false)
	if err != nil {
		t.Fatal(err)
	}
	if len([]rune(pw)) != 24 {
		t.Errorf("长度 = %d, want 24", len([]rune(pw)))
	}
	for _, set := range sets {
		if !strings.ContainsAny(pw, set) {
			t.Errorf("密码 %q 缺少 %q 里的字符", pw, set)
		}
	}
	if _, err := generate(12, nil, false); err == nil {
		t.Error("没有字符集时应当报错")
	}
	avoid, err := generate(64, []string{lowerChars, digitChars, "Il1O0o"}, true)
	if err != nil {
		t.Fatal(err)
	}
	if strings.ContainsAny(avoid, ambiguous) {
		t.Errorf("密码 %q 仍含易混淆字符", avoid)
	}
}

// TestPasswordBatch checks making several passwords at once, one a line.
func TestPasswordBatch(t *testing.T) {
	p := &Password{length: 12, count: 6, upper: true, lower: true, digits: true}
	p.gen()
	if len(p.pws) != 6 {
		t.Fatalf("生成 %d 条, want 6", len(p.pws))
	}
	lines := strings.Split(p.text, "\n")
	if len(lines) != 6 {
		t.Fatalf("文本 %d 行, want 6", len(lines))
	}
	for i, pw := range p.pws {
		if lines[i] != pw {
			t.Errorf("第 %d 行 = %q, want %q", i, lines[i], pw)
		}
		if n := len([]rune(pw)); n != 12 {
			t.Errorf("第 %d 条长度 = %d, want 12", i, n)
		}
	}
	// No set at all is told of, and leaves the batch shown as it was.
	p.upper, p.lower, p.digits = false, false, false
	p.gen()
	if p.err == "" || len(p.pws) != 6 {
		t.Errorf("没有字符集时应当报错并留着原来那批，得到 err=%q, %d 条", p.err, len(p.pws))
	}
}

// logoImage is a small picture to put in the middle of a code.
func logoImage() image.Image {
	img := image.NewRGBA(image.Rect(0, 0, 64, 64))
	for y := range 64 {
		for x := range 64 {
			img.Set(x, y, color.RGBA{R: 220, G: 40, B: 40, A: 255})
		}
	}
	return img
}

// TestQRCopyImage checks that the code can be copied as a picture, ready to
// paste into a chat, both by the button and by Cmd+Enter.
func TestQRCopyImage(t *testing.T) {
	for _, tc := range []struct {
		name string
		act  func(t *testing.T, tr *ui.Tester)
	}{
		{"按钮", func(t *testing.T, tr *ui.Tester) {
			if err := tr.Click("复制图片"); err != nil {
				t.Fatal(err)
			}
		}},
		{"Cmd+Enter", func(t *testing.T, tr *ui.Tester) { tr.Key(ui.Cmd, ui.KeyEnter) }},
	} {
		q := &QR{text: "https://example.com", size: 240}
		tr := ui.NewTester(func(c *ui.Context) { q.View(c) }, 900, 600)
		tc.act(t, tr)
		b, err := tr.ClipboardData().Read(transfer.PNG)
		if err != nil {
			t.Errorf("%s：剪贴板里没有图片：%v", tc.name, err)
			continue
		}
		if _, err := png.Decode(bytes.NewReader(b)); err != nil {
			t.Errorf("%s：复制出来的不是 PNG：%v", tc.name, err)
		}
	}
}

// whiteEdge is how many columns of the picture are white from its left.
func whiteEdge(img image.Image) int {
	b := img.Bounds()
	for x := range b.Dx() {
		for y := range b.Dy() {
			if r, _, _, _ := img.At(x, y).RGBA(); r != 0xffff {
				return x
			}
		}
	}
	return b.Dx()
}

// noisyPNG is a picture whose PNG is large enough that its Base64 is longer
// than what the tool shows of it at once.
func noisyPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 96, 96))
	rnd := rand.New(rand.NewSource(1))
	for i := range img.Pix {
		img.Pix[i] = uint8(rnd.Intn(256))
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// TestImage checks that a long Base64 is cut for showing while the whole of
// it stays for copying and saving, and what a save is called by default.
func TestImage(t *testing.T) {
	data := noisyPNG(t)
	im := &Image{}
	im.apply(pick{path: "/pics/photo.png", data: data})
	if im.err != "" {
		t.Fatalf("读图失败：%s", im.err)
	}
	if im.bitmap == nil || im.skip != "" {
		t.Errorf("这么小的图应当预览：bitmap=%v skip=%q", im.bitmap != nil, im.skip)
	}
	if want := base64.StdEncoding.EncodeToString(data); im.raw != want {
		t.Error("Base64 不是这个文件的内容")
	}
	if !strings.HasPrefix(im.uri, "data:image/png;base64,") {
		t.Errorf("Data URI 开头 = %q", im.uri[:30])
	}
	if len(im.raw) <= previewChars {
		t.Fatalf("这张图的 Base64 只有 %d 个字符，测不出截断", len(im.raw))
	}
	// What is shown is the head of the text and how long it is; what is
	// copied and saved is the whole of it.
	if !strings.HasPrefix(im.short[0], im.raw[:previewChars]+"…") {
		t.Errorf("显示的内容 = %.60q…", im.short[0])
	}
	if !strings.Contains(im.short[0], fmt.Sprintf("共 %d 个字符", len(im.raw))) {
		t.Errorf("应当说明总长，得到 %.80q", im.short[0])
	}
	if strings.Contains(im.raw, "…") || strings.Contains(im.uri, "…") {
		t.Error("留下来复制和保存的内容不该被截断")
	}
	if im.short[1] != widgets.Elide(im.uri, previewChars) {
		t.Error("Data URI 的显示与纯 Base64 的那份不一样")
	}
	if got := im.saveName(); got != "photo.png.base64.txt" {
		t.Errorf("默认文件名 = %q, want photo.png.base64.txt", got)
	}
	im.form = 1
	if got := im.saveName(); got != "photo.png.datauri.txt" {
		t.Errorf("Data URI 的默认文件名 = %q", got)
	}
	im.clear()
	if im.raw != "" || im.uri != "" || im.bitmap != nil || im.short != [2]string{} {
		t.Error("清空后不该还留着内容")
	}
}

// wrappedBase64 is s with a line break every 76 characters, as Base64 is
// wrapped in a file or a mail.
func wrappedBase64(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i += 76 {
		b.WriteString(s[i:min(i+76, len(s))])
		b.WriteByte('\n')
	}
	return b.String()
}

// TestImageDecode checks that a picture is read back out of the Base64 that
// holds it in each of the shapes one turns up in, and copied as a picture.
func TestImageDecode(t *testing.T) {
	file := noisyPNG(t)
	b64 := base64.StdEncoding.EncodeToString(file)
	for _, tc := range []struct{ name, text string }{
		{"纯 Base64", b64},
		{"折了行的 Base64", wrappedBase64(b64)},
		{"补全的 data URI", "data:image/png;base64," + b64},
		{"当成 JSON 字符串复制的", `"data:image/png;base64,` + b64 + `"`},
		{"没有填充的 Base64", base64.RawStdEncoding.EncodeToString(file)},
	} {
		im := &Image{dir: 1, text: tc.text}
		im.sync()
		if im.picErr != "" {
			t.Errorf("%s：%s", tc.name, im.picErr)
			continue
		}
		if im.pic == nil {
			t.Errorf("%s：没有读出图片", tc.name)
			continue
		}
		if !bytes.Equal(im.picData, file) {
			t.Errorf("%s：读出来的字节与原来那张图不同", tc.name)
		}
		if !strings.HasPrefix(im.picInfo, "PNG · 96×96 · ") {
			t.Errorf("%s：图片信息 = %q", tc.name, im.picInfo)
		}
		if len(im.shownText) >= len(tc.text) {
			t.Errorf("%s：显示的文本没有被截断（%d 个字符）", tc.name, len(im.shownText))
		}
	}

	// What is not Base64, and what is not a picture, are both told of.
	im := &Image{text: "这不是 Base64!!!"}
	im.sync()
	if im.picErr == "" || im.picData != nil {
		t.Errorf("不是 Base64 时应当报错，得到 %q", im.picErr)
	}
	im = &Image{text: base64.StdEncoding.EncodeToString([]byte("hello"))}
	im.sync()
	if im.picErr == "" || im.picData != nil {
		t.Errorf("不是图片时应当报错，得到 %q", im.picErr)
	}
	im.text = ""
	im.sync()
	if im.picErr != "" || im.pic != nil || im.shownText != "" {
		t.Errorf("清空后应当什么都不报，得到 %q", im.picErr)
	}

	// The picture goes on the clipboard as a PNG of its own size.
	im = &Image{dir: 1, text: "data:image/png;base64," + b64}
	tr := ui.NewTester(func(c *ui.Context) { im.View(c) }, 900, 700)
	if err := tr.Click("复制图片"); err != nil {
		t.Fatal(err)
	}
	got, err := tr.ClipboardData().Read(transfer.PNG)
	if err != nil {
		t.Fatalf("剪贴板里没有图片：%v", err)
	}
	img, err := png.Decode(bytes.NewReader(got))
	if err != nil {
		t.Fatalf("复制出来的不是 PNG：%v", err)
	}
	if p := img.Bounds().Size(); p.X != 96 || p.Y != 96 {
		t.Errorf("复制出来的图片是 %v, want 96×96", p)
	}
}

// TestQR checks making a code, changing its size, reading it back, and
// going on to read it with a logo in its middle.
func TestQR(t *testing.T) {
	const text = "https://example.com/hello"
	q := &QR{text: text, size: 240, level: "M"}
	q.sync()
	if q.code == nil || q.err != "" {
		t.Fatalf("生成失败：err=%q", q.err)
	}
	w240, _ := q.code.Size()
	if w240 == 0 {
		t.Fatal("二维码没有尺寸")
	}
	// The code fills the picture: only the quiet zone around it is white.
	if edge := whiteEdge(q.codeImage); edge*100/w240 > 10 {
		t.Errorf("边缘空白占 %d%%，太多了", edge*100/w240)
	}
	// The size takes effect at once, without the text changing.
	q.size = 120
	q.sync()
	if w120, _ := q.code.Size(); w120 >= w240 {
		t.Errorf("改小后 = %d, want 小于 %d", w120, w240)
	}
	q.size = 320
	q.sync()
	if w320, _ := q.code.Size(); w320 <= w240 {
		t.Errorf("改大后 = %d, want 大于 %d", w320, w240)
	}
	// What it holds reads back.
	if got, err := decodeQR(q.codeImage); err != nil || got != text {
		t.Errorf("读回 = %q, %v", got, err)
	}
	// A logo in the middle still reads, with the room a high level leaves.
	q.level, q.logo, q.logoImg = "H", "logo.png", logoImage()
	q.sync()
	if q.code == nil {
		t.Fatal("带 Logo 生成失败")
	}
	if got, err := decodeQR(q.codeImage); err != nil || got != text {
		t.Errorf("带 Logo 读回 = %q, %v", got, err)
	}
	// Nothing in, nothing made.
	q.text = ""
	q.sync()
	if q.code != nil {
		t.Error("清空内容后不应还留着二维码")
	}
}
