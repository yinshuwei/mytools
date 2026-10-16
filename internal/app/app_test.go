package app

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

// wants is something only one page shows, to tell that it built.
var wants = map[string]string{
	"json":  "树形",
	"codec": "编码方式",
	"clock": "日期时间 → 时间戳",
	"qr":    "纠错级别",
	"pw":    "密码",
	"img":   "选择图片…",
	"calc":  "复制结果",
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

// TestSettings checks the settings panel: the button in the corner and
// Cmd+, open it, Help holds the readme, About says what the toolbox is, and
// Escape closes it as a modal.
func TestSettings(t *testing.T) {
	a := New()
	tr := ui.NewTester(a.View, 1000, 760)
	tr.Key(ui.Cmd, ui.KeyComma)
	if !a.settings {
		t.Fatal("Cmd+, 没有打开设置")
	}
	tr.Key(0, ui.KeyEscape)
	if a.settings {
		t.Fatal("Esc 没有关掉设置")
	}
	if err := tr.Click("设置"); err != nil {
		t.Fatalf("找不到设置按钮：%v", err)
	}
	if !a.settings {
		t.Fatal("点了设置，面板没有打开")
	}
	if !strings.Contains(a.help, "# mytools") {
		t.Errorf("帮助里不是 readme 的内容：%d 个字符", len(a.help))
	}
	if err := tr.Click("关于"); err != nil {
		t.Fatal(err)
	}
	if !tr.HasText("MIT") || !tr.HasText("源码") {
		t.Errorf("关于里没有许可以及源码：%q", tr.Texts())
	}
	// The panel is sized to the window, so that what is in it is in it at
	// the smallest size the app allows as well.
	for _, size := range [][2]int{{1000, 760}, {720, 520}} {
		tr.SetSize(size[0], size[1])
		tr.Frame()
		done, ok := tr.Find("完成")
		if !ok {
			t.Fatalf("%dx%d：找不到完成按钮", size[0], size[1])
		}
		if done.Y+done.H > float32(size[1]) || done.Y < 0 {
			t.Errorf("%dx%d：完成按钮在窗口外（y=%.0f，高 %.0f）", size[0], size[1], done.Y, done.H)
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
