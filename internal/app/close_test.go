package app

import "testing"

// TestCloseWithoutTrayQuits checks that a machine with no tray to come back
// from leaves the program, rather than hiding a window nothing can raise.
func TestCloseWithoutTrayQuits(t *testing.T) {
	a := New()
	left := false
	a.quitApp = func() { left = true }
	a.hideToTray()
	if !left {
		t.Error("没有托盘时关掉窗口没有退出")
	}
	if !a.quitting {
		t.Error("退出时没有放行窗口关闭")
	}
}

// TestWindowToggle checks what the shortcut does to the window: one on screen
// goes away, one that is hidden or minimized comes back.
func TestWindowToggle(t *testing.T) {
	for _, c := range []struct {
		visible   bool
		minimized bool
		want      windowAction
	}{
		{true, false, actHide},
		{true, true, actShow},
		{false, false, actShow},
	} {
		if got := windowActionFor(c.visible, c.minimized); got != c.want {
			t.Errorf("显示=%v 最小化=%v 时 = %v，want %v", c.visible, c.minimized, got, c.want)
		}
	}
}

// TestTrayPlatforms checks what each platform can do with the icon: the menu
// is on the icon itself everywhere but a Mac, where it would take the click,
// and the click reaches the app everywhere but an AppIndicator.
func TestTrayPlatforms(t *testing.T) {
	for _, c := range []struct {
		goos   string
		menu   bool
		clicks bool
	}{
		{"windows", true, true},
		{"darwin", false, true},
		{"linux", true, false},
	} {
		if got := trayMenuAttached(c.goos); got != c.menu {
			t.Errorf("%s：菜单挂图标上 = %v，want %v", c.goos, got, c.menu)
		}
		if got := trayClickOpensWindow(c.goos); got != c.clicks {
			t.Errorf("%s：点击能到程序 = %v，want %v", c.goos, got, c.clicks)
		}
	}
}
