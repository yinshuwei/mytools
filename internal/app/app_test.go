package app

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// wants is something only one page shows, to tell that it built.
var wants = map[string]string{
	"json":  "JSON 结果",
	"codec": "编码方式",
	"clock": "日期时间 → 时间戳",
	"qr":    "纠错级别",
	"pw":    "密码",
	"img":   "选择图片…",
}

// TestPagesRender builds every page in a headless window: a page that
// breaks its layout or repeats a key panics or fails here.
func TestPagesRender(t *testing.T) {
	a := New()
	tr := ui.NewTester(a.View, 900, 700)
	for _, tl := range a.tools {
		a.selected = tl.ID()
		tr.Frame()
		if !tr.HasText(tl.Desc()) {
			t.Errorf("%s：没有显示自己的说明", tl.ID())
		}
		if want, ok := wants[tl.ID()]; ok && !tr.HasText(want) {
			t.Errorf("%s：没有显示 %q", tl.ID(), want)
		}
	}
}

// TestSidebarChooses checks that clicking an item shows its page.
func TestSidebarChooses(t *testing.T) {
	a := New()
	tr := ui.NewTester(a.View, 900, 700)
	a.selected = "json"
	tr.Frame()
	if err := tr.Click("随机密码"); err != nil {
		t.Fatalf("点击侧边栏：%v", err)
	}
	if a.selected != "pw" {
		t.Errorf("点击后选中 %q，want %q", a.selected, "pw")
	}
}
