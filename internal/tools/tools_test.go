package tools

import (
	"strings"
	"testing"
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
		if _, err := parseTime(in); err != nil {
			t.Errorf("parseTime(%q) = %v", in, err)
		}
	}
	if _, err := parseTime("不是时间"); err == nil {
		t.Error("parseTime 应当报错")
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
