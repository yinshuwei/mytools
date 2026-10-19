// Package widgets holds the small pieces every tool of the toolbox shares.
package widgets

import (
	"fmt"
	"runtime"
	"strings"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"
)

// ShortcutHint writes a shortcut as the platform writes it, for a tooltip:
// every shortcut of the toolbox holds the platform's command key, so
// ShortcutHint(0, "F") reads ⌘F on a Mac and Ctrl+F elsewhere, and extra
// modifiers are written before the key.
func ShortcutHint(extra ui.Modifiers, key string) string {
	var b strings.Builder
	if runtime.GOOS == "darwin" {
		b.WriteString("⌘")
		if extra&ui.Shift != 0 {
			b.WriteString("⇧")
		}
	} else {
		b.WriteString("Ctrl+")
		if extra&ui.Shift != 0 {
			b.WriteString("Shift+")
		}
	}
	b.WriteString(key)
	return b.String()
}

// zoom is how much larger or smaller the user asked the pages to be, 1 for
// the size the desktop asks for.
var zoom float32 = 1

// SetZoom tells the widgets how much larger the pages are to be. The app
// sets it from its appearance settings before it builds a frame; the theme
// carries the rest, but a size a page writes out itself does not go through
// the theme.
func SetZoom(z float32) {
	if z <= 0 {
		z = 1
	}
	zoom = z
}

// Scaled returns n DIPs at the zoom, for the sizes a page writes out itself:
// the text of a caption, and the icon beside it are both Scaled(12).
func Scaled(n float32) float32 { return n * zoom }

// Zoom is how much larger the pages are drawn, for a page of another kind
// that scales itself, such as the help page in its web view.
func Zoom() float32 { return zoom }

// Caption is a small bold label above a control.
func Caption(c *ui.Context, s string) ui.Element {
	return ui.Text(c, s).FontSize(Scaled(12)).Bold().TextColor(c.Theme().TextMuted)
}

// Hint is secondary text.
func Hint(c *ui.Context, s string) ui.Element {
	return ui.Text(c, s).FontSize(Scaled(12)).TextColor(c.Theme().TextMuted)
}

// Field stacks a caption above a control.
func Field(c *ui.Context, name string, build func()) {
	ui.Column(c).Gap(6).FillWidth().Children(func() {
		Caption(c, name)
		build()
	})
}

// FieldKey is Field with a key, for a field whose siblings come and go: the
// field then keeps its identity, and so what is being edited in it, the
// caret and the undo history included, as the siblings change.
func FieldKey(c *ui.Context, key any, name string, build func()) {
	ui.Column(c.Key(key)).Gap(6).FillWidth().Children(func() {
		Caption(c, name)
		build()
	})
}

// CopyButton copies s to the clipboard, saying so in a toast.
func CopyButton(c *ui.Context, label, s string) ui.Element {
	b := ui.Button(c, label)
	if b.Clicked() {
		c.WriteClipboard(s)
		c.Toast("已复制到剪贴板")
	}
	return b
}

// Result shows a read-only, selectable text area bound to s.
func Result(c *ui.Context, key any, label string, s *string, min, maxLines int) ui.Element {
	return ui.TextArea(c.Key(key), s).Label(label).ReadOnly(true).Lines(min, maxLines).FillWidth()
}

// Elide cuts s down to limit characters for showing it, ending what is left
// with an ellipsis and how much of s is not shown. Laying out a string of
// megabytes every frame is what makes a view hang, and what does not fit on
// screen is better copied or saved to a file than read.
func Elide(s string, limit int) string {
	total := utf8.RuneCountInString(s)
	if total <= limit {
		return s
	}
	// Cut on a rune boundary: the character at the edge may be several
	// bytes long.
	cut, n := len(s), 0
	for i := range s {
		if n == limit {
			cut = i
			break
		}
		n++
	}
	return fmt.Sprintf("%s…（共 %d 个字符，这里只显示前 %d 个）", s[:cut], total, limit)
}

// Drain moves everything a background goroutine sent, without blocking the
// frame that reads it.
func Drain[T any](ch chan T, apply func(T)) {
	for {
		select {
		case v := <-ch:
			apply(v)
		default:
			return
		}
	}
}
