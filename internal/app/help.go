package app

import (
	"fmt"
	stdhtml "html"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/egoist/mygo/ui"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"

	mytools "gitee.com/yinshuwei/mytools"
)

// helpAsText is set by MYTOOLS_NO_WEBVIEW, which asks for the readme as the
// text it is: for a machine where the web engine cannot draw at all.
var helpAsText = os.Getenv("MYTOOLS_NO_WEBVIEW") != ""

// helpBase is what the readme's relative links resolve against: the source
// tree, when the program runs from it, so that the screenshots it shows are
// there to be shown. A built binary run from anywhere else has none, and
// the readme shows as text is enough there.
func helpBase() string {
	wd, err := os.Getwd()
	if err != nil {
		return ""
	}
	if _, err := os.Stat(filepath.Join(wd, "docs")); err != nil {
		return ""
	}
	return "file://" + wd + "/"
}

// markdown renders the readme. The extensions are GitHub's, which is what
// the readme is written in: tables above all, and autolinks.
var markdown = goldmark.New(goldmark.WithExtensions(extension.GFM))

// helpBody is the readme as HTML, rendered once: only the page around it
// changes with the window's colors.
var helpBody = sync.OnceValue(func() string {
	var b strings.Builder
	if err := markdown.Convert([]byte(mytools.Readme), &b); err != nil {
		// Markdown that will not render is still worth reading.
		return "<pre>" + stdhtml.EscapeString(mytools.Readme) + "</pre>"
	}
	return b.String()
})

// helpDocument is the page the help panel shows: the readme, in a document
// dressed in the window's own colors, so that it reads as part of the
// window rather than as a page beside it.
func helpDocument(style string) string {
	var b strings.Builder
	b.WriteString(`<!doctype html><html lang="zh"><head><meta charset="utf-8">`)
	b.WriteString(`<meta name="viewport" content="width=device-width, initial-scale=1">`)
	b.WriteString("<style>")
	b.WriteString(style)
	b.WriteString(helpCSS)
	b.WriteString("</style></head><body>")
	b.WriteString(helpBody())
	b.WriteString("</body></html>")
	return b.String()
}

// helpStyle is the window's colors as CSS, which is what the page is loaded
// with: loading it again is what a change of colors costs.
func helpStyle(t *ui.Theme) string {
	scheme := "light"
	if t.Dark {
		scheme = "dark"
	}
	return fmt.Sprintf(":root{color-scheme:%s;--bg:%s;--surface:%s;--text:%s;--muted:%s;"+
		"--border:%s;--accent:%s;--on-accent:%s}",
		scheme, cssColor(t.Background), cssColor(t.Surface), cssColor(t.Text),
		cssColor(t.TextMuted), cssColor(t.Border), cssColor(t.Accent), cssColor(t.AccentText))
}

// cssColor writes a window color the way CSS does.
func cssColor(c ui.Color) string {
	if c.A == 255 {
		return fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B)
	}
	return fmt.Sprintf("rgba(%d,%d,%d,%.2f)", c.R, c.G, c.B, float32(c.A)/255)
}

// helpCSS dresses the rendered readme, in the colors helpStyle sets.
const helpCSS = `
*{box-sizing:border-box}
body{
	margin:0;padding:20px 24px 40px;
	background:var(--bg);color:var(--text);overflow-wrap:break-word;
	font:14px/1.75 -apple-system,BlinkMacSystemFont,"Segoe UI","Noto Sans CJK SC",
		"PingFang SC","Hiragino Sans GB","Microsoft YaHei",sans-serif;
	-webkit-font-smoothing:antialiased;
}
h1,h2,h3,h4{line-height:1.3;margin:1.6em 0 .6em;font-weight:600}
h1{font-size:23px;margin-top:0}
h2{font-size:19px;padding-bottom:.32em;border-bottom:1px solid var(--border)}
h3{font-size:16px}
h4{font-size:14px}
p,ul,ol,blockquote,table,pre,dl{margin:.85em 0}
ul,ol{padding-left:1.6em}
li{margin:.2em 0}
a{color:var(--accent);text-decoration:none}
a:hover{text-decoration:underline}
code{
	font:12.5px/1.5 ui-monospace,SFMono-Regular,Menlo,Consolas,"Noto Sans Mono CJK SC",monospace;
	background:var(--surface);border:1px solid var(--border);border-radius:5px;padding:1px 5px;
}
pre{
	background:var(--surface);border:1px solid var(--border);border-radius:8px;
	padding:12px 14px;overflow:auto;
}
pre code{background:none;border:0;padding:0;font-size:12.5px}
table{border-collapse:collapse;width:100%;font-size:13.5px}
th,td{border:1px solid var(--border);padding:7px 10px;text-align:left;vertical-align:top}
th{background:var(--surface);font-weight:600;white-space:nowrap}
blockquote{margin-left:0;padding:.1em 0 .1em 14px;border-left:3px solid var(--border);color:var(--muted)}
hr{border:0;border-top:1px solid var(--border);margin:1.8em 0}
img{max-width:100%}
::selection{background:var(--accent);color:var(--on-accent)}
`
