package app

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/yinshuwei/mytools/internal/widgets"
)

// wants is something only one page shows, to tell that it built.
var wants = map[string]string{
	"json":  "树形",
	"codec": "编码方式",
	"clock": "日期时间 → 时间戳",
	"qr":    "纠错级别",
	"pw":    "密码",
	"img":   "选择图片…",
	"shot":  "开始截图",
	"calc":  "复制结果",
	"git":   "选择 Root 目录…",
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

// TestSettings checks the settings page: the button in the corner and Cmd+,
// open it, Help holds the readme, About says what the toolbox is, Escape
// leaves it, and so does picking a tool in the sidebar.
func TestSettings(t *testing.T) {
	a := New()
	tr := ui.NewTester(a.View, 1000, 760)
	tr.Key(ui.Cmd, ui.KeyComma)
	if !a.settings {
		t.Fatal("Cmd+, 没有打开设置")
	}
	tr.Key(0, ui.KeyEscape)
	if a.settings {
		t.Fatal("Esc 没有离开设置")
	}
	if err := tr.Click("设置"); err != nil {
		t.Fatalf("找不到设置按钮：%v", err)
	}
	if !a.settings {
		t.Fatal("点了设置，页面没有换过去")
	}
	if !tr.HasText("帮助与关于") {
		t.Errorf("标题里没有说明这是设置：%q", tr.Texts())
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
	// The button leaves it as well, and the window is at the smallest size
	// the app allows while it is up.
	tr.SetSize(720, 520)
	tr.Frame()
	if err := tr.Click("回到工具"); err != nil {
		t.Fatalf("找不到回到工具：%v", err)
	}
	if a.settings {
		t.Error("「回到工具」没有离开设置")
	}
	// And picking a tool is leaving them: the page the sidebar names is
	// what the window is for.
	a.settings = true
	tr.Frame()
	if err := tr.Click("随机密码"); err != nil {
		t.Fatal(err)
	}
	if a.settings || a.selected != "pw" {
		t.Errorf("点侧边栏后 settings=%v，选中 %q", a.settings, a.selected)
	}
	if !tr.HasText("按字符集批量生成随机密码") {
		t.Errorf("显示的不是随机密码页：%q", tr.Texts())
	}
	// A click on the tool already picked counts as well: the sidebar picks
	// pages, and setting the same one again still shows it.
	a.settings = true
	tr.Frame()
	if a.selected != a.picked {
		t.Fatalf("这一条要先落在这个前提上：选中的就是已选中的那个（%q / %q）", a.selected, a.picked)
	}
	if err := tr.Click("随机密码"); err != nil {
		t.Fatal(err)
	}
	if a.settings {
		t.Error("点已经选中的那个工具，没有离开设置")
	}
	if !tr.HasText("按字符集批量生成随机密码") {
		t.Errorf("没有回到随机密码页：%q", tr.Texts())
	}
}

// TestAppearance checks the appearance settings: the choice is kept for the
// next run, the page renders, and the zoom reaches the pages.
func TestAppearance(t *testing.T) {
	old := settingsFile
	settingsFile = filepath.Join(t.TempDir(), "settings.json")
	t.Cleanup(func() { settingsFile = old })

	a := New()
	if a.themeMode != "system" || a.zoom != 1 {
		t.Fatalf("默认外观 = %q, %v", a.themeMode, a.zoom)
	}
	a.themeMode, a.zoom = "dark", 1.25
	a.saveSettings()

	next := New()
	next.loadSettings()
	if next.themeMode != "dark" || next.zoom != 1.25 {
		t.Errorf("读回来的外观 = %q, %v", next.themeMode, next.zoom)
	}

	next.settings, next.panel = true, 0
	tr := ui.NewTester(next.View, 1000, 720)
	tr.Frame()
	if !tr.HasText("缩放比例") || !tr.HasText("跟随系统") {
		t.Errorf("外观页没有出现：%q", tr.Texts())
	}
	if got := widgets.Scaled(10); got != 12.5 {
		t.Errorf("缩放后的字号 = %v，want 12.5", got)
	}
	// Choosing a color from the page keeps it as well.
	if err := tr.Click("浅色"); err != nil {
		t.Fatal(err)
	}
	if next.themeMode != "light" {
		t.Errorf("点了浅色后 themeMode = %q", next.themeMode)
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
