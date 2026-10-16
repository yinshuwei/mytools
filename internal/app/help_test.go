package app

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

// darkTheme is a theme to dress a page in, standing for a window's.
var darkTheme = &ui.Theme{
	Dark:       true,
	Background: ui.Hex("#1e1e1e"),
	Surface:    ui.Hex("#2a2a2a"),
	Text:       ui.Hex("#e8e8e8"),
	TextMuted:  ui.Hex("#9a9a9a"),
	Border:     ui.Hex("#3a3a3a"),
	Accent:     ui.Hex("#4c8dff"),
	AccentText: ui.Hex("#ffffff"),
}

// TestHelpDocument checks that the readme is rendered as a page: its
// markdown becomes HTML, and the page wears the window's colors.
func TestHelpDocument(t *testing.T) {
	page := helpDocument(helpStyle(darkTheme))
	for _, want := range []string{
		"<!doctype html>",
		"<h1",        // a heading of the readme
		"目录结构",       // a section of it
		"<table>",    // its tables
		"<pre><code", // its code blocks
		"<a href=",   // its links
		"color-scheme:dark",
		"--bg:#1e1e1e",
		"--accent:#4c8dff",
	} {
		if !strings.Contains(page, want) {
			t.Errorf("页面里没有 %q", want)
		}
	}
	light := &ui.Theme{Background: ui.Hex("#ffffff"), Text: ui.Hex("#111111")}
	if got := helpStyle(light); !strings.Contains(got, "color-scheme:light") {
		t.Errorf("浅色主题 = %q", got)
	}
}

// TestCSSColor checks how a window color is written for CSS.
func TestCSSColor(t *testing.T) {
	for _, tc := range []struct {
		c    ui.Color
		want string
	}{
		{ui.Hex("#1e1e1e"), "#1e1e1e"},
		{ui.Hex("#4c8dff"), "#4c8dff"},
		{ui.RGBA(10, 20, 30, 0.4), "rgba(10,20,30,0.40)"},
		{ui.RGBA(255, 255, 255, 0), "rgba(255,255,255,0.00)"},
	} {
		if got := cssColor(tc.c); got != tc.want {
			t.Errorf("cssColor(%v) = %q, want %q", tc.c, got, tc.want)
		}
	}
}

// TestHelpFallsBackToText checks that help still shows something on a
// machine where no web view can be had: the readme as the text it is, and
// the reason it is not a page.
func TestHelpFallsBackToText(t *testing.T) {
	a := New() // no window, so no web view to be made
	tr := ui.NewTester(a.View, 1000, 760)
	a.settings = true
	tr.Frame()
	if a.helpWeb != nil {
		t.Fatal("没有窗口，却建出了 web view")
	}
	if !tr.HasText("web view") {
		t.Errorf("应当说明为什么显示的是原文：%q", tr.Texts())
	}
	if !strings.Contains(a.help, "# mytools") {
		t.Errorf("退回来的应当还是 readme 的原文")
	}
}
