package tools

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/transfer"
	"github.com/egoist/mygo/ui"
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
// paste into a chat.
func TestQRCopyImage(t *testing.T) {
	q := &QR{text: "https://example.com", size: 240}
	tr := ui.NewTester(func(c *ui.Context) { q.View(c) }, 900, 600)
	if err := tr.Click("复制图片"); err != nil {
		t.Fatal(err)
	}
	b, err := tr.ClipboardData().Read(transfer.PNG)
	if err != nil {
		t.Fatalf("剪贴板里没有图片：%v", err)
	}
	if _, err := png.Decode(bytes.NewReader(b)); err != nil {
		t.Errorf("复制出来的不是 PNG：%v", err)
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
