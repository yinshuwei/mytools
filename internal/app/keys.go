package app

import (
	"runtime"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/yinshuwei/mytools/internal/widgets"
)

// The two actions a shortcut can be put on, by the names they are kept under.
const (
	keyShot   = "shot"
	keyWindow = "window"
)

// applyShortcuts registers the shortcuts the settings hold, and has the
// screenshot page say what came of its own.
func (a *App) applyShortcuts() {
	a.dropShortcuts()
	a.keyErr = map[string]string{}
	a.registerKey(keyShot, a.shotKey, a.TakeShot)
	a.registerKey(keyWindow, a.showKey, a.showWindow)
	a.decorateShot()
}

// dropShortcuts lets go of the keys this app holds, so that they can be set
// again without the app finding itself in the way.
func (a *App) dropShortcuts() {
	for _, acc := range []string{a.shotKey, a.showKey} {
		if acc != "" {
			mygo.GlobalShortcut.Unregister(acc)
		}
	}
}

// registerKey puts acc on fn, and keeps why it could not be done, if it could
// not: a combination another program holds is the usual reason.
func (a *App) registerKey(which, acc string, fn func()) {
	if acc == "" {
		return
	}
	if which == keyWindow && acc == a.shotKey {
		a.keyErr[which] = "这个组合已经给了截图"
		return
	}
	if err := mygo.GlobalShortcut.Register(acc, fn); err != nil {
		a.keyErr[which] = err.Error()
	}
}

// setShortcut hands acc to one of the two actions. The key it replaces is let
// go first and taken back if the desktop refuses the new one, so that a
// combination already spoken for does not leave the action with nothing.
func (a *App) setShortcut(which, acc string) {
	cur, other, fn := &a.shotKey, a.showKey, a.TakeShot
	if which == keyWindow {
		cur, other, fn = &a.showKey, a.shotKey, a.showWindow
	}
	if acc != "" && acc == other {
		a.keyErr[which] = "这个组合已经给了另一个动作"
		return
	}
	old := *cur
	if old != "" {
		mygo.GlobalShortcut.Unregister(old)
	}
	*cur = acc
	if acc != "" {
		if err := mygo.GlobalShortcut.Register(acc, fn); err != nil {
			*cur = old
			if old != "" {
				_ = mygo.GlobalShortcut.Register(old, fn)
			}
			a.keyErr[which] = err.Error()
			return
		}
	}
	delete(a.keyErr, which)
	a.decorateShot()
	a.saveSettings()
}

// decorateShot has the screenshot page name the key that takes a picture, or
// say why the one it was given did not take.
func (a *App) decorateShot() {
	if a.shotTool == nil {
		return
	}
	a.shotTool.Hotkey = prettyAccel(a.shotKey)
	a.shotTool.HotkeyProblem = ""
	if msg, bad := a.keyErr[keyShot]; bad {
		a.shotTool.HotkeyProblem = "全局快捷键 " + prettyAccel(a.shotKey) +
			" 注册失败（可能被别的程序占用了）：" + msg + "；页面上的按钮仍然可以截图。"
	}
}

// keysPage is where the global shortcuts are set: the key that takes a
// picture, and the key that calls the window back when the program is away.
func (a *App) keysPage(c *ui.Context, t *ui.Theme) {
	ui.Column(c).Gap(14).FillWidth().Children(func() {
		widgets.Field(c, "截图", func() { a.keyRow(c, t, keyShot, "截图", a.shotKey) })
		widgets.Field(c, "显示主窗口", func() {
			a.keyRow(c, t, keyWindow, "显示主窗口", a.showKey)
		})
		widgets.Hint(c, "程序在后台时也生效。点「修改」后按下组合键即可，Esc 取消；组合里至少带一个修饰键（Ctrl / Alt / Shift / ⌘），F1–F12 可以单独用。")
		widgets.Hint(c, "「清除」就是不要这个快捷键。被别的程序占用了会保留原来那一套，并在这里说明。")
	})
}

// keyRow is one shortcut: what it is now, the button that records another,
// and the button that clears it.
func (a *App) keyRow(c *ui.Context, t *ui.Theme, which, name, acc string) {
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		if a.recording == which {
			box := ui.Box(c.Key("record"+which)).Padding(6, 10).Radius(widgets.Scaled(6)).
				Background(t.SurfaceHover).Border(1, t.Accent).
				Focusable().
				HandleInput(func(ev ui.InputEvent) bool { return a.captureKey(which, ev) })
			box.Children(func() {
				ui.Text(c, "按下组合键…（Esc 取消）").FontSize(widgets.Scaled(12))
			})
			if a.recordFocus {
				a.recordFocus = false
				box.Focus()
			}
			return
		}
		text, muted := prettyAccel(acc), false
		if text == "" {
			text, muted = "未设置", true
		}
		label := ui.Text(c, text).FontSize(widgets.Scaled(13)).MinWidth(widgets.Scaled(120))
		if muted {
			label.TextColor(t.TextMuted)
		}
		if ui.Button(c.Key("set"+which), "修改").Label(name + "：修改").
			Tooltip("按下要用的组合键").Clicked() {
			a.recording, a.recordFocus = which, true
		}
		if ui.Button(c.Key("clear"+which), "清除").Label(name + "：清除").
			Disabled(acc == "").Clicked() {
			a.setShortcut(which, "")
		}
	})
	if msg := a.keyErr[which]; msg != "" {
		ui.Text(c, "没生效："+msg).FontSize(widgets.Scaled(12)).TextColor(t.Danger)
	}
}

// captureKey takes the key pressed for the shortcut being recorded.
func (a *App) captureKey(which string, ev ui.InputEvent) bool {
	if ev.Kind != ui.InputKeyDown {
		return false
	}
	if ev.Key == ui.KeyEscape && ev.Mods == 0 {
		a.recording = ""
		return true
	}
	acc, ok := accelFromKey(ev.Key, ev.Mods)
	if !ok {
		return true // a modifier alone, or a key accelerators have no name for
	}
	a.recording = ""
	a.setShortcut(which, acc)
	return true
}

// accelFromKey turns a key pressed into the accelerator mygo takes, modifiers
// first. A letter or a digit on its own is none: a global shortcut for it
// would be taken from every program.
func accelFromKey(k ui.Key, mods ui.Modifiers) (string, bool) {
	name, ok := accelName(k)
	if !ok {
		return "", false
	}
	var parts []string
	if mods&ui.Super != 0 {
		parts = append(parts, "Super")
	}
	if mods&ui.Ctrl != 0 {
		parts = append(parts, "Ctrl")
	}
	if mods&ui.Alt != 0 {
		parts = append(parts, "Alt")
	}
	if mods&ui.Shift != 0 {
		parts = append(parts, "Shift")
	}
	if len(parts) == 0 && len(name) == 1 {
		return "", false
	}
	// The accelerator's own form capitalises a single character, and the
	// settings keep that form, so that one key compares equal to another.
	if len(name) == 1 {
		name = strings.ToUpper(name)
	}
	return strings.Join(append(parts, name), "+"), true
}

// accelName is the name an accelerator gives a key, and whether it has one.
func accelName(k ui.Key) (string, bool) {
	switch k {
	case ui.KeyEnter:
		return "Enter", true
	case ui.KeyTab:
		return "Tab", true
	case ui.KeySpace:
		return "Space", true
	case ui.KeyBackspace:
		return "Backspace", true
	case ui.KeyDelete:
		return "Delete", true
	case ui.KeyInsert:
		return "Insert", true
	case ui.KeyHome:
		return "Home", true
	case ui.KeyEnd:
		return "End", true
	case ui.KeyPageUp:
		return "PageUp", true
	case ui.KeyPageDown:
		return "PageDown", true
	case ui.KeyLeft:
		return "Left", true
	case ui.KeyRight:
		return "Right", true
	case ui.KeyUp:
		return "Up", true
	case ui.KeyDown:
		return "Down", true
	case ui.KeyMinus:
		return "-", true
	case ui.KeyEqual:
		return "=", true
	case ui.KeyComma:
		return ",", true
	case ui.KeyPeriod:
		return ".", true
	case ui.KeySlash:
		return "/", true
	case ui.KeySemicolon:
		return ";", true
	case ui.KeyQuote:
		return "'", true
	case ui.KeyBracketLeft:
		return "[", true
	case ui.KeyBracketRight:
		return "]", true
	case ui.KeyBackslash:
		return "\\", true
	case ui.KeyBackquote:
		return "`", true
	}
	if name, ok := digitName(k); ok {
		return name, true
	}
	if name, ok := letterName(k); ok {
		return name, true
	}
	if k >= ui.KeyF1 && k <= ui.KeyF12 {
		return "F" + itoa(int(k-ui.KeyF1)+1), true
	}
	return "", false
}

// digitName is the digit a key types, if it types one.
func digitName(k ui.Key) (string, bool) {
	if k >= ui.Key0 && k <= ui.Key9 {
		return itoa(int(k - ui.Key0)), true
	}
	return "", false
}

// letterName is the letter a key types, if it types one.
func letterName(k ui.Key) (string, bool) {
	if k >= ui.KeyA && k <= ui.KeyZ {
		return string(rune('a' + int(k-ui.KeyA))), true
	}
	return "", false
}

func itoa(n int) string {
	if n < 10 {
		return string(rune('0' + n))
	}
	return string(rune('0'+n/10)) + string(rune('0'+n%10))
}

// prettyAccel writes an accelerator the way the platform's people write it:
// ⌘⇧A on a Mac, Ctrl+Alt+A elsewhere.
func prettyAccel(acc string) string { return prettyAccelFor(acc, runtime.GOOS) }

func prettyAccelFor(acc, goos string) string {
	if acc == "" {
		return ""
	}
	parts := strings.Split(acc, "+")
	key := parts[len(parts)-1]
	var out []string
	for _, m := range parts[:len(parts)-1] {
		switch strings.ToLower(m) {
		case "super", "cmd", "command", "meta":
			if goos == "darwin" {
				out = append(out, "⌘")
			} else if goos == "windows" {
				out = append(out, "Win")
			} else {
				out = append(out, "Super")
			}
		case "ctrl", "control":
			if goos == "darwin" {
				out = append(out, "⌃")
			} else {
				out = append(out, "Ctrl")
			}
		case "alt", "option":
			if goos == "darwin" {
				out = append(out, "⌥")
			} else {
				out = append(out, "Alt")
			}
		case "shift":
			if goos == "darwin" {
				out = append(out, "⇧")
			} else {
				out = append(out, "Shift")
			}
		}
	}
	if len(key) == 1 {
		key = strings.ToUpper(key)
	}
	if goos == "darwin" {
		return strings.Join(append(out, key), "")
	}
	return strings.Join(append(out, key), "+")
}
