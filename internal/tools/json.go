package tools

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"

	"gitee.com/yinshuwei/mytools/internal/widgets"
)

// JSON formats and validates JSON, and shows it as a tree that folds, is
// colored by value type, and copies any block of it.
type JSON struct {
	input  string
	indent string
	mode   int // 0 the tree, 1 the text

	root   *jnode
	flat   []row           // the lines shown: those whose ancestors are open
	closed map[string]bool // the paths of the containers that are folded
	list   ui.ListState
	src    string // the input parsed last

	out    string // the whole document as text, made when it is asked for
	outKey string // what out is of: the input and the indent

	nodes int
	depth int
	err   string
}

func (*JSON) ID() string   { return "json" }
func (*JSON) Name() string { return "JSON 美化" }
func (*JSON) Desc() string { return "格式化、高亮、折叠，按块复制" }

// FillsHeight takes the whole content area: the input and the tree scroll
// apart.
func (*JSON) FillsHeight() bool { return true }

// jsonRowHeight is the height of a row of the tree.
const jsonRowHeight = 24

// jnode is one value of the document, in the order it was written.
type jnode struct {
	key   string // the member's name in its parent, "" when it is an array's
	path  string // the node's identity, as $.user.tags[2]
	kind  byte   // '{' or '[' for containers, 's', 'n', 'b' or 'z' otherwise
	text  string // a scalar as JSON, its string already quoted
	raw   string // a string's content without its quotes
	kids  []*jnode
	depth int
	index int  // the item's place in its array, for an array's
	item  bool // whether the node is an element of an array
}

func (n *jnode) container() bool { return n.kind == '{' || n.kind == '[' }

// closeBrace is the brace that ends a container of n.
func (n *jnode) closeBrace() string {
	if n.kind == '[' {
		return "]"
	}
	return "}"
}

// row is one line of the tree: a value, or the brace closing a container
// whose values show.
type row struct {
	node  *jnode
	close bool
}

// key is the line's identity, which the list follows as lines move.
func (r row) key() string {
	if r.close {
		return r.node.path + "/}"
	}
	return r.node.path
}

// quoteJSON writes s as a JSON string, keeping what is printable as it is.
func quoteJSON(s string) string {
	var b strings.Builder
	b.Grow(len(s) + 2)
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		case '\b':
			b.WriteString(`\b`)
		case '\f':
			b.WriteString(`\f`)
		default:
			if r < 0x20 {
				fmt.Fprintf(&b, `\u%04x`, r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}

// parseJSON reads src into a tree that keeps the order of the members.
func parseJSON(src string) (*jnode, error) {
	dec := json.NewDecoder(strings.NewReader(src))
	dec.UseNumber()
	root, err := decodeValue(dec, "", "$", 0)
	if err != nil {
		return nil, err
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		if err == nil {
			return nil, errors.New("JSON 结束后还有多余的内容")
		}
		return nil, err
	}
	return root, nil
}

// decodeValue reads one value, and the values inside a container.
func decodeValue(dec *json.Decoder, key, path string, depth int) (*jnode, error) {
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	n := &jnode{key: key, path: path, depth: depth}
	switch v := tok.(type) {
	case json.Delim:
		if v != '{' && v != '[' {
			return nil, fmt.Errorf("意外的 %q", v)
		}
		n.kind = byte(v)
		for dec.More() {
			childKey, childPath := "", ""
			if v == '{' {
				kt, err := dec.Token()
				if err != nil {
					return nil, err
				}
				k, ok := kt.(string)
				if !ok {
					return nil, errors.New("对象的键不是字符串")
				}
				childKey = k
				// Quoted, so that no two members share a path whatever
				// their names are.
				childPath = path + "." + quoteJSON(k)
			} else {
				childKey = strconv.Itoa(len(n.kids))
				childPath = path + "[" + childKey + "]"
			}
			child, err := decodeValue(dec, childKey, childPath, depth+1)
			if err != nil {
				return nil, err
			}
			if v == '[' {
				child.item, child.index = true, len(n.kids)
			}
			n.kids = append(n.kids, child)
		}
		if _, err := dec.Token(); err != nil { // the closing brace
			return nil, err
		}
		return n, nil
	case string:
		n.kind, n.text, n.raw = 's', quoteJSON(v), v
	case json.Number:
		n.kind, n.text = 'n', v.String()
	case bool:
		n.kind, n.text = 'b', strconv.FormatBool(v)
	case nil:
		n.kind, n.text = 'z', "null"
	default:
		return nil, fmt.Errorf("意外的值 %v", tok)
	}
	return n, nil
}

// writePretty writes the value with indentation.
func (n *jnode) writePretty(b *strings.Builder, depth int, tab string) {
	switch n.kind {
	case '{', '[':
		open, close := "{", "}"
		if n.kind == '[' {
			open, close = "[", "]"
		}
		if len(n.kids) == 0 {
			b.WriteString(open + close)
			return
		}
		b.WriteString(open)
		b.WriteByte('\n')
		for i, k := range n.kids {
			b.WriteString(strings.Repeat(tab, depth+1))
			if n.kind == '{' {
				b.WriteString(quoteJSON(k.key))
				b.WriteString(": ")
			}
			k.writePretty(b, depth+1, tab)
			if i < len(n.kids)-1 {
				b.WriteByte(',')
			}
			b.WriteByte('\n')
		}
		b.WriteString(strings.Repeat(tab, depth))
		b.WriteString(close)
	default:
		b.WriteString(n.text)
	}
}

// writeCompact writes the value on one line.
func (n *jnode) writeCompact(b *strings.Builder) {
	switch n.kind {
	case '{', '[':
		open, close := "{", "}"
		if n.kind == '[' {
			open, close = "[", "]"
		}
		b.WriteString(open)
		for i, k := range n.kids {
			if i > 0 {
				b.WriteByte(',')
			}
			if n.kind == '{' {
				b.WriteString(quoteJSON(k.key))
				b.WriteByte(':')
			}
			k.writeCompact(b)
		}
		b.WriteString(close)
	default:
		b.WriteString(n.text)
	}
}

// walkAll calls fn for every node of the tree, folded ones included.
func walkAll(n *jnode, fn func(*jnode)) {
	fn(n)
	for _, k := range n.kids {
		walkAll(k, fn)
	}
}

// indentString is the whitespace the chosen indentation is made of.
func (j *JSON) indentString() string {
	switch j.indent {
	case "4":
		return "    "
	case "Tab":
		return "\t"
	default:
		return "  "
	}
}

// sync parses the input again when it changed.
func (j *JSON) sync() {
	if j.input == j.src {
		return
	}
	j.src = j.input
	src := strings.TrimSpace(j.input)
	if src == "" {
		j.root, j.flat, j.err, j.nodes, j.depth = nil, nil, "", 0, 0
		return
	}
	root, err := parseJSON(src)
	if err != nil {
		j.root, j.flat = nil, nil
		j.err = err.Error()
		return
	}
	j.root, j.err, j.outKey = root, "", ""
	j.nodes, j.depth = 0, 0
	walkAll(root, func(n *jnode) {
		j.nodes++
		j.depth = max(j.depth, n.depth)
	})
	j.rebuild()
}

// rebuild flattens the tree into the lines that show: the nodes whose
// ancestors are all open, each open container followed by its closing
// brace. It makes a new slice, so that a frame building the old lines
// keeps them as they were.
func (j *JSON) rebuild() {
	rows := make([]row, 0, max(len(j.flat), 16))
	var walk func(n *jnode)
	walk = func(n *jnode) {
		rows = append(rows, row{node: n})
		if !n.container() || j.closed[n.path] {
			return
		}
		for _, k := range n.kids {
			walk(k)
		}
		if len(n.kids) > 0 {
			rows = append(rows, row{node: n, close: true})
		}
	}
	if j.root != nil {
		walk(j.root)
	}
	j.flat = rows
}

// folded reports whether the container of n is closed.
func (j *JSON) folded(n *jnode) bool { return j.closed[n.path] }

// toggle folds or unfolds the container of n.
func (j *JSON) toggle(n *jnode) {
	if j.closed == nil {
		j.closed = map[string]bool{}
	}
	if j.closed[n.path] {
		delete(j.closed, n.path)
	} else {
		j.closed[n.path] = true
	}
	j.rebuild()
}

// foldBelow folds, or unfolds, n and everything under it.
func (j *JSON) foldBelow(n *jnode, open bool) {
	if j.closed == nil {
		j.closed = map[string]bool{}
	}
	walkAll(n, func(m *jnode) {
		if !m.container() {
			return
		}
		if open {
			delete(j.closed, m.path)
		} else {
			j.closed[m.path] = true
		}
	})
	j.rebuild()
}

// unfoldAll opens every container.
func (j *JSON) unfoldAll() {
	j.closed = nil
	j.rebuild()
}

// foldAll closes every container but the root, so that the top level shows.
func (j *JSON) foldAll() {
	if j.root == nil {
		return
	}
	j.closed = map[string]bool{}
	walkAll(j.root, func(n *jnode) {
		if n.container() && n != j.root {
			j.closed[n.path] = true
		}
	})
	j.rebuild()
}

// block returns n as text, with the chosen indentation: what copying a
// block puts on the clipboard.
func (j *JSON) block(n *jnode) string {
	var b strings.Builder
	n.writePretty(&b, 0, j.indentString())
	return b.String()
}

// text returns the whole document as text.
func (j *JSON) text() string {
	if j.root == nil {
		return strings.TrimSpace(j.input)
	}
	if key := j.input + "\x00" + j.indent; j.outKey != key {
		j.outKey, j.out = key, j.block(j.root)
	}
	return j.out
}

// compact returns the whole document on one line.
func (j *JSON) compact() string {
	if j.root == nil {
		return strings.TrimSpace(j.input)
	}
	var b strings.Builder
	j.root.writeCompact(&b)
	return b.String()
}

// valueColor is what a value of n shows in.
func valueColor(t *ui.Theme, n *jnode) ui.Color {
	switch n.kind {
	case 's':
		return t.Success
	case 'n':
		return t.Warning
	case 'b':
		return t.Danger
	default:
		return t.TextMuted
	}
}

func (j *JSON) View(c *ui.Context) {
	t := c.Theme()
	if j.indent == "" {
		j.indent = "2"
	}
	j.sync()

	ui.Column(c).FillWidth().FillHeight().Gap(10).Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			widgets.Hint(c, "缩进")
			ui.Select(c.Key("indent"), &j.indent, []string{"2", "4", "Tab"}).Width(88)
			if j.root != nil {
				widgets.Hint(c, fmt.Sprintf("%d 个值 · %d 层", j.nodes, j.depth+1))
			}
			ui.Spacer(c)
			widgets.CopyButton(c.Key("copyall"), "复制全部", j.text())
			if ui.Button(c.Key("min"), "复制压缩版").Disabled(j.root == nil).Clicked() {
				c.WriteClipboard(j.compact())
				c.Toast("已复制压缩版")
			}
			if ui.Button(c.Key("clear"), "清空").Clicked() {
				j.input, j.src, j.outKey = "", "", ""
			}
		})
		if j.err != "" {
			ui.Text(c, "错误："+j.err).FontSize(12).TextColor(t.Danger)
		}
		ui.Row(c).Grow(1).FillWidth().AlignItems(ui.Stretch).Children(func() {
			ui.Column(c).Grow(1).MinWidth(0).Gap(6).Padding(0, 14, 0, 0).Children(func() {
				widgets.Caption(c, "输入")
				ui.TextArea(c.Key("in"), &j.input).Label("JSON 输入").
					Placeholder(`{"hello": "world"}`).FillWidth().Grow(1)
			})
			ui.Column(c).Grow(1).MinWidth(0).Gap(6).
				Padding(0, 0, 0, 14).BorderWidth(0, 0, 0, 1).BorderColor(t.Border).Children(func() {
				ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
					widgets.Caption(c, "结果")
					ui.Segmented(c.Key("mode"), &j.mode, "树形", "文本")
					ui.Spacer(c)
					if j.mode == 0 {
						if ui.Button(c.Key("unfold"), "展开全部").Disabled(j.root == nil).Clicked() {
							j.unfoldAll()
						}
						if ui.Button(c.Key("fold"), "折叠全部").Disabled(j.root == nil).Clicked() {
							j.foldAll()
						}
					}
				})
				switch {
				case j.mode == 1:
					ui.TextArea(c.Key("out"), &j.out).Label("JSON 结果").ReadOnly(true).FillWidth().Grow(1)
				case j.root != nil:
					rows := j.flat
					j.list.Key = func(i int) any { return rows[i].key() }
					ui.List(c, &j.list, len(rows), func(i int) { j.row(c, rows, i) }).
						Label("JSON 树").FillWidth().Grow(1)
				default:
					ui.Column(c).Grow(1).Center().Children(func() {
						widgets.Hint(c, "输入 JSON 后自动解析")
					})
				}
			})
		})
	})
}

// row builds one line of the tree: a value, with its fold arrow, its name
// and a button copying it, or the brace closing a container.
func (j *JSON) row(c *ui.Context, rows []row, i int) {
	if i < 0 || i >= len(rows) {
		return
	}
	n, closing := rows[i].node, rows[i].close
	t := c.Theme()
	line := ui.Row(c).Height(jsonRowHeight).FillWidth().AlignItems(ui.Center).Gap(4).PaddingX(4)
	if line.Hovered() {
		line.Background(t.Surface)
	}
	if closing {
		// The brace lines up with the name of its container.
		line.Children(func() {
			if n.depth > 0 {
				ui.Box(c).Width(float32(n.depth) * 14).Shrink(0)
			}
			ui.Box(c).Width(14).Shrink(0)
			ui.Text(c, n.closeBrace()).TextColor(t.TextMuted).SingleLine()
		})
		return
	}
	line.ContextMenu(func(m *ui.Menu) {
		if m.Item("复制此块").Chosen() {
			j.copyBlock(c, n)
		}
		// A member of an object has a name of its own; a string has a
		// content without the quotes around it.
		if !n.item && n.key != "" {
			if m.Item("复制名字").Chosen() {
				c.WriteClipboard(n.key)
				c.Toast("已复制名字")
			}
		}
		if n.kind == 's' {
			if m.Item("复制字符串内容").Chosen() {
				c.WriteClipboard(n.raw)
				c.Toast("已复制字符串内容")
			}
		}
		if n.container() {
			m.Separator()
			if m.Item("展开全部子级").Chosen() {
				j.foldBelow(n, true)
			}
			if m.Item("折叠全部子级").Chosen() {
				j.foldBelow(n, false)
			}
		}
	})
	line.Children(func() {
		if n.depth > 0 {
			ui.Box(c).Width(float32(n.depth) * 14).Shrink(0)
		}
		if n.container() {
			arrow := "▾"
			if j.folded(n) {
				arrow = "▸"
			}
			b := ui.ButtonBase(c.Key("fold")).Padding(0, 3).Shrink(0).Tooltip("折叠或展开")
			b.Children(func() { ui.Text(c, arrow).FontSize(10).TextColor(t.TextMuted) })
			if b.Clicked() {
				j.toggle(n)
			}
		} else {
			ui.Box(c).Width(14).Shrink(0)
		}
		if n.item {
			ui.Text(c, strconv.Itoa(n.index)+":").TextColor(t.TextMuted).SingleLine().Shrink(0)
		} else if n.key != "" {
			ui.Text(c, quoteJSON(n.key)+":").TextColor(t.Accent).SingleLine().Shrink(0).Tooltip(n.path)
		}
		switch {
		case n.container():
			open := "{"
			if n.kind == '[' {
				open = "["
			}
			if j.folded(n) {
				ui.Text(c, open+"…"+n.closeBrace()).TextColor(t.TextMuted).SingleLine().Shrink(0)
				widgets.Hint(c, fmt.Sprintf("%d 项", len(n.kids)))
			} else {
				ui.Text(c, open).TextColor(t.TextMuted).SingleLine().Shrink(0)
			}
		default:
			ui.Text(c, n.text).TextColor(valueColor(t, n)).SingleLine().Tooltip(n.path)
		}
		ui.Spacer(c)
		if ui.Button(c.Key("copy"), "复制").FontSize(11).Padding(0, 6).Shrink(0).Clicked() {
			j.copyBlock(c, n)
		}
	})
}

// copyBlock puts the block of n on the clipboard.
func (j *JSON) copyBlock(c *ui.Context, n *jnode) {
	c.WriteClipboard(j.block(n))
	c.Toast("已复制此块")
}
