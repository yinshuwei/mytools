package widgets

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// TestElide checks that what fits is left as it is, and that what does not is
// cut where a character ends, with how much was left out said.
func TestElide(t *testing.T) {
	const wide = "一二三四五"
	if got := Elide(wide, 5); got != wide {
		t.Errorf("刚好放得下时 = %q, want %q", got, wide)
	}
	got := Elide(wide, 3)
	if !utf8.ValidString(got) {
		t.Fatalf("截断处切开了一个字符：%q", got)
	}
	if !strings.HasPrefix(got, "一二三…") {
		t.Errorf("截断后 = %q, want 以「一二三…」开头", got)
	}
	if !strings.Contains(got, "共 5 个字符") {
		t.Errorf("应当说明总长，得到 %q", got)
	}

	long := strings.Repeat("A", 4000)
	if got := Elide(long, 600); !strings.HasPrefix(got, strings.Repeat("A", 600)+"…") {
		t.Errorf("长串截断 = %q", got[:80])
	}
	if strings.Contains(Elide(long, 600), strings.Repeat("A", 601)) {
		t.Error("截断后不该还有第 601 个字符")
	}
}
