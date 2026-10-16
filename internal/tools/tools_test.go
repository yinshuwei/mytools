package tools

import (
	"strings"
	"testing"
)

// TestJSON checks formatting and minifying.
func TestJSON(t *testing.T) {
	j := &JSON{input: `{"b":1,"a":[1,2]}`, indent: "2"}
	j.run(false)
	if want := "{\n  \"b\": 1,\n  \"a\": [\n    1,\n    2\n  ]\n}"; j.output != want {
		t.Errorf("格式化 = %q, want %q", j.output, want)
	}
	m := &JSON{input: j.output}
	m.run(true)
	if m.output != `{"b":1,"a":[1,2]}` {
		t.Errorf("压缩 = %q", m.output)
	}
	bad := &JSON{input: "{oops"}
	bad.run(false)
	if bad.err == "" || bad.output != "" {
		t.Errorf("无效 JSON 应当报错，得到 err=%q out=%q", bad.err, bad.output)
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
