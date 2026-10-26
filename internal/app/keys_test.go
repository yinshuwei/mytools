package app

import (
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
)

// TestAccelFromKey checks the keys a recorded shortcut is made of: modifiers
// first, a letter or a digit only with one of them, and the keys an
// accelerator has no name for left out.
func TestAccelFromKey(t *testing.T) {
	for _, c := range []struct {
		key  ui.Key
		mods ui.Modifiers
		want string
		ok   bool
	}{
		{ui.KeyA, ui.Ctrl | ui.Alt, "Ctrl+Alt+A", true},
		{ui.KeyA, ui.Shift | ui.Ctrl, "Ctrl+Shift+A", true},
		{ui.Key1, ui.Ctrl | ui.Shift, "Ctrl+Shift+1", true},
		{ui.KeySpace, ui.Super, "Super+Space", true},
		{ui.KeyComma, ui.Ctrl, "Ctrl+,", true},
		{ui.KeyF5, 0, "F5", true},
		{ui.KeyA, 0, "", false},
		{ui.Key1, 0, "", false},
		{ui.KeyUnknown, ui.Ctrl, "", false},
	} {
		got, ok := accelFromKey(c.key, c.mods)
		if got != c.want || ok != c.ok {
			t.Errorf("accelFromKey(%v, %v) = %q，%v，want %q，%v", c.key, c.mods, got, ok, c.want, c.ok)
		}
	}
}

// TestPrettyAccel checks the way a shortcut is written for people, which
// follows the platform.
func TestPrettyAccel(t *testing.T) {
	for _, c := range []struct{ acc, goos, want string }{
		{"Ctrl+Alt+A", "linux", "Ctrl+Alt+A"},
		{"Ctrl+Alt+A", "windows", "Ctrl+Alt+A"},
		{"Super+Shift+A", "darwin", "⌘⇧A"},
		{"Ctrl+Alt+A", "darwin", "⌃⌥A"},
		{"Super+Space", "windows", "Win+Space"},
		{"F5", "linux", "F5"},
		{"", "windows", ""},
	} {
		if got := prettyAccelFor(c.acc, c.goos); got != c.want {
			t.Errorf("漂亮写法(%q, %s) = %q，want %q", c.acc, c.goos, got, c.want)
		}
	}
}

// TestKeysKept checks that the shortcuts survive a run, defaults and all, and
// that one cleared on purpose stays cleared.
func TestKeysKept(t *testing.T) {
	old := settingsFile
	settingsFile = filepath.Join(t.TempDir(), "settings.json")
	t.Cleanup(func() { settingsFile = old })

	a := New()
	a.loadSettings()
	if a.shotKey != defaultShotKey() || a.showKey != "" {
		t.Fatalf("默认快捷键 = %q，%q", a.shotKey, a.showKey)
	}
	a.shotKey, a.showKey = "Ctrl+Shift+9", "Ctrl+Alt+M"
	a.saveSettings()

	next := New()
	next.loadSettings()
	if next.shotKey != "Ctrl+Shift+9" || next.showKey != "Ctrl+Alt+M" {
		t.Errorf("读回来的快捷键 = %q，%q", next.shotKey, next.showKey)
	}

	next.shotKey, next.showKey = "", ""
	next.saveSettings()
	third := New()
	third.loadSettings()
	if third.shotKey != "" || third.showKey != "" {
		t.Errorf("清空后读回来 = %q，%q，want 都空", third.shotKey, third.showKey)
	}
}

// TestKeysPage checks the settings page of the shortcuts: both actions are
// there with what they hold now, and 「修改」 starts recording.
func TestKeysPage(t *testing.T) {
	a := New()
	a.shotKey, a.showKey = "Ctrl+Alt+A", ""
	a.settings, a.panel = true, panelKeys
	tr := ui.NewTester(a.View, 1000, 720)
	tr.Frame()
	if !tr.HasText("显示 / 隐藏窗口") || !tr.HasText("未设置") || !tr.HasText("程序在后台时也生效") {
		t.Fatalf("快捷键页没有出现：%q", tr.Texts())
	}
	if err := tr.Click("截图：修改"); err != nil {
		t.Fatal(err)
	}
	if a.recording != keyShot {
		t.Errorf("点修改后 recording = %q，want %q", a.recording, keyShot)
	}
	tr.Frame()
	if !tr.HasText("按下组合键") {
		t.Errorf("开始录制后没有提示：%q", tr.Texts())
	}
	// Escape calls the recording off without leaving the page.
	tr.Key(0, ui.KeyEscape)
	tr.Frame()
	if a.recording != "" || !a.settings {
		t.Errorf("Esc 后 recording = %q，settings = %v", a.recording, a.settings)
	}
}
