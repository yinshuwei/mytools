package tools

import (
	"runtime"

	"github.com/egoist/mygo/ui"

	"github.com/yinshuwei/mytools/internal/widgets"
)

// Screenshot is the page that starts a screenshot. The shot itself is a
// window of its own over the screen, so this page only has the button, the
// shortcut, and what to expect.
type Screenshot struct {
	// Shot starts a screenshot. The app puts it in place once it has a
	// window to hide and capture.
	Shot func()
	// Hotkey is how the global shortcut is written for people, e.g.
	// Ctrl+Alt+A.
	Hotkey string
	// Problem is why screenshots cannot be taken on this machine or
	// session.
	Problem string
	// HotkeyProblem is why the global shortcut could not be registered;
	// the button still works.
	HotkeyProblem string
}

func (*Screenshot) ID() string   { return "shot" }
func (*Screenshot) Name() string { return "截图" }
func (*Screenshot) Desc() string { return "框选屏幕区域，复制或保存为 PNG" }

func (*Screenshot) MaxWidth() float32 { return 720 }

func (s *Screenshot) View(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Gap(12).Children(func() {
		ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
			start := ui.PrimaryButton(c, "开始截图")
			if s.Shot == nil || s.Problem != "" || s.HotkeyProblem != "" {
				start = start.Disabled(true)
			}
			if start.Clicked() && s.Shot != nil {
				s.Shot()
			}
			if s.Hotkey != "" {
				widgets.Hint(c, "全局快捷键 "+s.Hotkey)
			}
		})
		if s.Problem != "" {
			ui.Text(c, s.Problem).FontSize(widgets.Scaled(12)).TextColor(t.Danger)
		}
		if s.HotkeyProblem != "" {
			ui.Text(c, s.HotkeyProblem).FontSize(widgets.Scaled(12)).TextColor(t.Danger)
		}
		ui.Divider(c)
		ui.Text(c, "拖动鼠标框选一块区域，然后：").
			FontSize(widgets.Scaled(13))
		ui.Column(c).Gap(4).Children(func() {
			for _, line := range []string{
				"Enter 或 " + copyKeyText() + "：复制到剪贴板",
				saveKeyText() + "：保存为 PNG（默认落在图片文件夹）",
				"Esc 或右键：取消",
			} {
				ui.Text(c, "· "+line).FontSize(widgets.Scaled(12)).
					TextColor(t.TextMuted)
			}
		})
		ui.Text(c, "只截取框选的像素，不做窗口识别。多块屏幕时，每块屏幕各有一层，一次框选一块。").
			FontSize(widgets.Scaled(12)).TextColor(t.TextMuted)
		if runtime.GOOS == "darwin" {
			ui.Text(c, "第一次使用需要在系统设置 → 隐私与安全性 → 屏幕录制里允许本应用，然后重新启动它。").
				FontSize(widgets.Scaled(12)).TextColor(t.TextMuted)
		}
	})
}

func copyKeyText() string {
	if runtime.GOOS == "darwin" {
		return "⌘C"
	}
	return "Ctrl+C"
}

func saveKeyText() string {
	if runtime.GOOS == "darwin" {
		return "⌘S"
	}
	return "Ctrl+S"
}
