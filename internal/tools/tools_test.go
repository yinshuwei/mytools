package tools

import (
	"strings"
	"testing"
	"time"

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

// names is the names of the rows shown, for telling what a search kept.
func names(j *JSON) string {
	var b strings.Builder
	for _, r := range j.flat {
		if !r.close {
			b.WriteString(" " + r.node.key)
		}
	}
	return b.String()
}

// TestJSONFind checks that looking narrows the tree to what matches and the
// path to it.
func TestJSONFind(t *testing.T) {
	j := &JSON{input: `{"user":{"name":"Ada","city":"Paris"},"count":3,"other":{"name":"Bob"}}`}
	j.sync()
	if j.err != "" {
		t.Fatal(j.err)
	}
	all := len(j.flat)

	j.query = "name"
	j.sync()
	if j.matches != 2 {
		t.Errorf("匹配数 = %d, want 2", j.matches)
	}
	if len(j.flat) >= all {
		t.Errorf("查找后行数 = %d，应当比原来的 %d 少", len(j.flat), all)
	}
	// The containers on the way to a match show too.
	if !strings.Contains(names(j), " user") || !strings.Contains(names(j), " other") {
		t.Errorf("应当显示到匹配的路径，得到 %q", names(j))
	}
	// Looking for a container keeps what is inside it.
	j.query = "user"
	j.sync()
	if j.matches != 1 {
		t.Errorf("匹配数 = %d, want 1", j.matches)
	}
	if !strings.Contains(names(j), " name") || !strings.Contains(names(j), " city") {
		t.Errorf("匹配到的容器里的内容应当都在，得到 %q", names(j))
	}
	// A value is found by its text, and what is beside it is not.
	j.query = "Bob"
	j.sync()
	if j.matches != 1 {
		t.Errorf("按值查找 = %d 个匹配，want 1", j.matches)
	}
	if !strings.Contains(names(j), " other") || strings.Contains(names(j), " user") {
		t.Errorf("只该留下 Bob 那条路径，得到 %q", names(j))
	}
	// Nothing found leaves nothing to show.
	j.query = "nowhere*at*all"
	j.sync()
	if len(j.flat) != 0 || j.matches != 0 {
		t.Errorf("无匹配时不该有行，得到 %d 行 %d 匹配", len(j.flat), j.matches)
	}
	// Clearing the query brings the whole document back.
	j.query = ""
	j.sync()
	if len(j.flat) != all {
		t.Errorf("清空查找后行数 = %d, want %d", len(j.flat), all)
	}
	// Folding while looking holds, and shows again once the query is gone.
	j.query = "Ada"
	j.sync()
	j.foldAll()
	j.query = ""
	j.sync()
	if len(j.flat) != 5 {
		t.Errorf("清空查找后应当回到折叠状态（5 行），得到 %d", len(j.flat))
	}
	if !j.flat[1].folded || j.flat[1].node.key != "user" {
		t.Errorf("user 应当仍然折叠，得到 %+v", j.flat[1])
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

// TestQR checks that a code is made, and dropped when the text is.
func TestQR(t *testing.T) {
	q := &QR{text: "https://example.com"}
	q.sync()
	if q.bitmap == nil || q.err != "" {
		t.Fatalf("生成失败：err=%q bitmap=%v", q.err, q.bitmap)
	}
	if w, h := q.bitmap.Size(); w == 0 || h == 0 || w != h {
		t.Errorf("二维码尺寸异常：%dx%d", w, h)
	}
	q.text = ""
	q.sync()
	if q.bitmap != nil {
		t.Error("清空内容后不应还留着二维码")
	}
}
