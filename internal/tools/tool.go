// Package tools holds the pages of the toolbox.
package tools

import (
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// Tool is one page of the toolbox.
type Tool interface {
	// ID names the page, and is what a sidebar selection holds.
	ID() string
	// Name and Desc label the page.
	Name() string
	Desc() string
	// View builds the page for one frame.
	View(c *ui.Context)
}

// A WindowAware tool acts on the window, and is given it once the app has
// made one.
type WindowAware interface {
	SetWindow(*mygo.Window)
}

// A FillsHeight tool takes the whole content area, instead of being
// scrolled as one column: it lays out and scrolls its own panes.
type FillsHeight interface {
	FillsHeight() bool
}

// PageWidth is the width a page of controls is laid out in at most. A page
// of a tree, as JSON 美化 is, asks for no such thing: it takes the whole
// content area, however wide.
const PageWidth float32 = 1000

// A MaxWidth tool asks for a page no wider than the DIPs it names, at the
// left of the content area: stretched across a very wide window, the buttons
// at its right are a journey from what they act on.
type MaxWidth interface {
	MaxWidth() float32
}

// All returns the tools of the toolbox, in the order of the sidebar.
func All() []Tool {
	return []Tool{
		&JSON{},
		&Codec{},
		&Clock{},
		&QR{},
		&Password{},
		&Image{},
		&Calc{},
		&Git{},
	}
}
