package app

import (
	"time"

	"github.com/egoist/mygo/ui"
)

// toastFade is how long a toast takes to come up and to go.
const toastFade = 180 * time.Millisecond

// toasts builds the window's toasts along the top edge, in place of the
// bottom one MyGo's viewport would use: a page keeps its controls in the
// lower half of the window, so a toast over them covers the very row that
// just said it — the copy button of a result box, say. The newest toast is
// the highest, the stack growing down from the edge, and the toasts slide
// in and out from above rather than fade in place.
func (a *App) toasts(c *ui.Context, t *ui.Theme) {
	// The look of the theme's own toasts, which shows over any page.
	fill, text := t.Inverse, t.InverseText
	if fill == ui.Transparent {
		fill = t.Text
	}
	if text == ui.Transparent {
		text = t.Background
	}
	width, _ := c.Size()
	move := ui.ElementTransition{
		Duration: toastFade,
		Enter:    &ui.Motion{Y: -t.Space(2)},
		Exit:     &ui.Motion{Y: -t.Space(2)},
	}
	ui.ToastViewportBase(c, func(viewport ui.Element, toasts []ui.Toast) {
		viewport.Top(t.Space(6)).Justify(ui.Start).
			AlignItems(ui.Center).Gap(t.Space(2))
		for i := len(toasts) - 1; i >= 0; i-- {
			ts := toasts[i]
			p := ui.ToastBase(c, ts)
			box := p.Root.Row().AlignItems(ui.Center).Gap(t.Space(4)).
				Padding(t.Space(2.5), t.Space(4)).Radius(t.Space(2)).
				MaxWidth(width - t.Space(12)).
				Background(fill).TextColor(text).Transition(move)
			box.Shadow(0, 6, 20, 0, ui.RGBA(0, 0, 0, 0.25))
			box.Children(func() {
				if ts.Description == "" {
					ui.Text(c, ts.Title)
				} else {
					ui.Column(c).Shrink(1).Gap(t.Space(0.5)).Children(func() {
						ui.Text(c, ts.Title).FontWeight(600)
						ui.Text(c, ts.Description).TextColor(text.Alpha(0.75))
					})
				}
				if ts.Action == "" {
					return
				}
				b := p.ActionButton().Padding(t.Space(1), t.Space(2.5)).
					Radius(t.Radius).TextColor(t.Accent.Mix(text, 0.35)).FontWeight(600)
				if b.Hovered() {
					b.Background(text.Alpha(0.12))
				}
				b.Children(func() { ui.Text(c, ts.Action) })
			})
		}
	})
}
