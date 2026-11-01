package app

import "testing"

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

// TestTrayMenu checks the icon's menu: the window goes away from it, and it
// holds the one way out of the program that closing the window does not.
func TestTrayMenu(t *testing.T) {
	a := New()
	hides, quits := false, false
	for _, it := range a.trayMenu().Items() {
		switch it.Label {
		case "隐藏主窗口":
			hides = true
		case "退出":
			quits = it.Click != nil
		}
	}
	if !hides {
		t.Error("托盘菜单里没有隐藏主窗口")
	}
	if !quits {
		t.Error("托盘菜单里没有退出")
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
