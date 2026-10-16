// Package widgets holds the small pieces every tool of the toolbox shares.
package widgets

import "github.com/egoist/mygo/ui"

// Caption is a small bold label above a control.
func Caption(c *ui.Context, s string) ui.Element {
	return ui.Text(c, s).FontSize(12).Bold().TextColor(c.Theme().TextMuted)
}

// Hint is secondary text.
func Hint(c *ui.Context, s string) ui.Element {
	return ui.Text(c, s).FontSize(12).TextColor(c.Theme().TextMuted)
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
