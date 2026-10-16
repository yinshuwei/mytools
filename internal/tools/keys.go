package tools

import "github.com/egoist/mygo/ui"

// The shortcuts every page answers to, so that they mean the same thing
// wherever one is. Each page also names them in the tooltips of the buttons
// they press, with widgets.ShortcutHint.
//
//	Cmd+Enter   the page's main button
//	Cmd+O       the button that opens a file dialog
//	Cmd+F       the search, in the page that has one
//	Esc         clear the search
//	Cmd+S       save what a page has to a file

// mainPressed reports whether the page's main button was pressed: by a click
// or by Cmd+Enter, the key one tries first.
func mainPressed(c *ui.Context, e ui.Element) bool {
	return e.Clicked() || c.Shortcut(ui.Cmd, ui.KeyEnter)
}

// filePressed reports whether the button that opens a file dialog was
// pressed: by a click or by Cmd+O. Where that button is also the page's
// main one, ask mainPressed for Cmd+Enter beside it, so that a page with
// another main button does not open a dialog on it too.
func filePressed(c *ui.Context, e ui.Element) bool {
	return e.Clicked() || c.Shortcut(ui.Cmd, ui.KeyO)
}
