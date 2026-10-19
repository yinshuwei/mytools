package tools

import "github.com/egoist/mygo/ui"

// gitIcon parses one icon of the git page: a 16×16 drawing in strokes of
// the text color, as the theme's icons are.
func gitIcon(paths string) *ui.SVG {
	return ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.3" stroke-linecap="round" stroke-linejoin="round">` + paths + `</svg>`))
}

var (
	icoGitFolder  = gitIcon(`<path d="M1.6 4.6a1 1 0 0 1 1-1h3.1l1.4 1.6h5.3a1 1 0 0 1 1 1v6.2a1 1 0 0 1-1 1H2.6a1 1 0 0 1-1-1z"/>`)
	icoCaretRight = gitIcon(`<path d="M6 3.8 10.2 8 6 12.2"/>`)
	icoCaretDown  = gitIcon(`<path d="M3.8 6 8 10.2 12.2 6"/>`)
	// icoGitRepo is a branch and its two ends.
	icoGitRepo  = gitIcon(`<circle cx="4.6" cy="3.6" r="1.5"/><circle cx="4.6" cy="12.4" r="1.5"/><circle cx="11.4" cy="5.8" r="1.5"/><path d="M4.6 5.1v5.8M11.4 7.3c0 2.6-3.2 2.3-5.9 3.1"/>`)
	icoPull     = gitIcon(`<path d="M8 2.6v8.2m0 0L5 7.8m3 3 3-3M3 13.4h10"/>`)
	icoPush     = gitIcon(`<path d="M8 13.4V5.2m0 0L5 8.2m3-3 3 3M3 2.6h10"/>`)
	icoTerminal = gitIcon(`<rect x="1.8" y="2.8" width="12.4" height="10.4" rx="1.6"/><path d="M4.6 6.4 6.8 8.4l-2.2 2M8.6 10.7h2.9"/>`)
	icoCode     = gitIcon(`<path d="M5.6 4.2 1.9 8l3.7 3.8M10.4 4.2 14.1 8l-3.7 3.8"/>`)
	// icoRefresh is a circle with an arrow, as a status read again.
	icoRefresh = gitIcon(`<path d="M13.2 8a5.2 5.2 0 1 1-1.5-3.7"/><path d="M13.2 2.6v3.1H10"/>`)
	icoClose   = gitIcon(`<path d="M4.5 4.5l7 7M11.5 4.5l-7 7"/>`)
)
