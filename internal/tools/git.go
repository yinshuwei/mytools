package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"gitee.com/yinshuwei/mytools/internal/widgets"
)

const (
	// gitRowHeight is the height of a repository or a folder of them.
	gitRowHeight = 26
	// gitIndent is how much a level of folders indents what is under it.
	gitIndent = 14
)

// Git lists the git repositories under the folders one adds, each with what
// it has to be committed, pulled or pushed, and opens one where it is worked
// on: a terminal, the file manager, or an editor the machine has.
type Git struct {
	win *mygo.Window
	svc ui.Services

	roots []*gitRoot // one per tab, in the order they were added
	tab   int        // the tab shown

	// editorName is the editor chosen last run, as the button that opens a
	// repository names it; empty takes the first editor the machine has.
	editorName string

	picking bool
	picks   chan gitPicked
	scans   chan gitScanDone
	acts    chan gitActDone
}

// gitRoot is one folder added, and what was found under it.
type gitRoot struct {
	path string

	scanning bool          // a scan is under way
	err      string        // why the last scan failed
	took     time.Duration // how long it took
	scanned  time.Time
	repos    []*gitRepo

	q         string          // what the search holds
	collapsed map[string]bool // the folders folded away, by relative path
	rows      []gitRow        // what the list shows this frame
	list      ui.ListState
}

// gitRow is a line of the list: a folder of repositories, or one of them.
type gitRow struct {
	group  string   // the folder's path relative to the root
	depth  int      // how deep it is, for the indent
	count  int      // repositories under a folder
	folded bool     // a folder whose contents are hidden
	repo   *gitRepo // nil for a folder
}

// key names the row across frames, so that the list keeps its place.
func (r gitRow) key() any {
	if r.repo != nil {
		return "repo|" + r.repo.path
	}
	return "dir|" + r.group
}

// gitPicked is what the folder dialog returned: where, or why not. An empty
// path is a dialog the user closed.
type gitPicked struct {
	path string
	err  error
}

// gitScanDone is what a scan of one root found, or, when only the statuses
// were read again, the statuses by path.
type gitScanDone struct {
	root     string
	repos    []*gitRepo
	statuses map[string]gitStatus
	err      error
	took     time.Duration
}

// gitActDone is what a pull or a push came to, and the status it left.
type gitActDone struct {
	root   string
	path   string
	action string
	out    string
	err    error
	status gitStatus
}

func (*Git) ID() string   { return "git" }
func (*Git) Name() string { return "Git 代码库" }
func (*Git) Desc() string {
	return "扫描多个 Root 下所有的 git 代码库，看状态、拉取、推送与打开"
}

// MaxWidth keeps the list of repositories from stretching across a wide
// window, where the buttons at its right sit far from the names.
func (*Git) MaxWidth() float32 { return PageWidth }

// FillsHeight takes the whole content area: the list of repositories scrolls
// by itself, under the controls that do not.
func (*Git) FillsHeight() bool { return true }

// SetWindow is what WindowAware asks of a tool acting on the window. The
// roots the last run had open come back with it, once: a window of the
// program is the point at which the page is really shown.
func (g *Git) SetWindow(win *mygo.Window) {
	g.win = win
	g.loadConfig()
}

// root returns the tab of a path, or nil when the root is gone.
func (g *Git) root(path string) *gitRoot {
	for _, r := range g.roots {
		if r.path == path {
			return r
		}
	}
	return nil
}

// repo returns the repository at path in a root, or nil.
func (r *gitRoot) repo(path string) *gitRepo {
	for _, rp := range r.repos {
		if rp.path == path {
			return rp
		}
	}
	return nil
}

// pick asks for a folder, in the background, so that the window keeps
// drawing.
func (g *Git) pick(c *ui.Context) {
	if g.picking {
		return
	}
	if g.win == nil {
		c.Toast("窗口还没有建好")
		return
	}
	g.picking = true
	win, svc, ch := g.win, c.Services(), g.picks
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Parent:      win,
			Title:       "选择 Root 目录",
			ButtonLabel: "选择",
			Directory:   true,
		})
		switch {
		case err != nil:
			ch <- gitPicked{err: err}
		case len(paths) == 0:
			ch <- gitPicked{}
		default:
			ch <- gitPicked{path: filepath.Clean(paths[0])}
		}
		svc.Invalidate()
	}()
}

// addRoot takes what the folder dialog returned.
func (g *Git) addRoot(c *ui.Context, p gitPicked) {
	g.picking = false
	switch {
	case p.err != nil:
		c.Toast("选择目录失败：" + p.err.Error())
	case p.path == "":
		// Canceled.
	default:
		for i, r := range g.roots {
			if r.path == p.path {
				g.tab = i
				return
			}
		}
		r := &gitRoot{path: p.path, collapsed: map[string]bool{}}
		g.roots = append(g.roots, r)
		g.tab = len(g.roots) - 1
		g.saveConfig()
		g.scan(r)
	}
}

// removeRoot closes a tab; nothing on disk is touched. The tab shown stays
// the same root, unless it is the one closed.
func (g *Git) removeRoot(i int) {
	if i < 0 || i >= len(g.roots) {
		return
	}
	if i < g.tab {
		g.tab--
	}
	g.roots = append(g.roots[:i], g.roots[i+1:]...)
	if g.tab >= len(g.roots) {
		g.tab = len(g.roots) - 1
	}
	if g.tab < 0 {
		g.tab = 0
	}
	g.saveConfig()
}

// scan finds the repositories under a root again, in the background.
func (g *Git) scan(r *gitRoot) {
	if r == nil || r.scanning {
		return
	}
	r.scanning, r.err = true, ""
	svc, ch, path := g.svc, g.scans, r.path
	if ch == nil {
		r.scanning = false
		return
	}
	go func() {
		start := time.Now()
		repos, err := scanGitRoots(path)
		ch <- gitScanDone{root: path, repos: repos, err: err, took: time.Since(start)}
		svc.Invalidate()
	}()
}

// refreshStatuses reads the statuses of the repositories of a root again,
// without walking the tree: what a rescan does, minus the finding.
func (g *Git) refreshStatuses(r *gitRoot) {
	if r == nil || r.scanning || len(r.repos) == 0 {
		return
	}
	r.scanning, r.err = true, ""
	svc, ch := g.svc, g.scans
	root := r.path
	paths := make([]string, len(r.repos))
	for i, rp := range r.repos {
		paths[i] = rp.path
	}
	go func() {
		start := time.Now()
		statuses := readStatuses(paths)
		byPath := make(map[string]gitStatus, len(paths))
		for i, p := range paths {
			byPath[p] = statuses[i]
		}
		ch <- gitScanDone{root: root, statuses: byPath, took: time.Since(start)}
		svc.Invalidate()
	}()
}

// applyScan takes what a scan found, or the statuses a refresh read.
func (g *Git) applyScan(d gitScanDone) {
	r := g.root(d.root)
	if r == nil {
		return
	}
	r.scanning, r.took, r.scanned = false, d.took, time.Now()
	if d.err != nil {
		r.err = d.err.Error()
		return
	}
	r.err = ""
	if d.statuses != nil {
		// A refresh only says what the repositories are like now: the rows
		// keep their place, and a pull or a push still running stays so.
		for path, st := range d.statuses {
			rp := r.repo(path)
			if rp == nil {
				continue
			}
			if st.err != "" {
				rp.err = st.err
			} else {
				rp.gitStatus = st
			}
		}
		return
	}
	r.repos = d.repos
}

// run pulls or pushes one repository, in the background, and reads its
// status again once git is done.
func (g *Git) run(r *gitRoot, rp *gitRepo, action string) {
	if rp.op != "" {
		return
	}
	rp.op, rp.msg = action, ""
	svc, ch := g.svc, g.acts
	root, path, branch, upstream := r.path, rp.path, rp.branch, rp.upstream
	go func() {
		out, err := runGitAction(path, action, branch, upstream)
		ch <- gitActDone{
			root: root, path: path, action: action, out: out, err: err,
			status: readGitStatus(path),
		}
		svc.Invalidate()
	}()
}

// refresh reads one repository's status again, in the background.
func (g *Git) refresh(r *gitRoot, rp *gitRepo) {
	if rp.op != "" {
		return
	}
	rp.op, rp.msg = "刷新", ""
	svc, ch := g.svc, g.acts
	root, path := r.path, rp.path
	go func() {
		ch <- gitActDone{root: root, path: path, action: "refresh", status: readGitStatus(path)}
		svc.Invalidate()
	}()
}

// applyAct takes what a pull, a push or a refresh came to.
func (g *Git) applyAct(c *ui.Context, d gitActDone) {
	r := g.root(d.root)
	if r == nil {
		return
	}
	rp := r.repo(d.path)
	if rp == nil {
		return
	}
	rp.op = ""
	if d.action == "refresh" {
		// A refresh only says what the repository is like now; a status
		// that could not be read keeps the branch it knew and says why.
		if d.status.err != "" {
			rp.err = d.status.err
		} else {
			rp.gitStatus = d.status
		}
		return
	}
	if d.status.err == "" {
		rp.gitStatus = d.status
	} else if rp.err == "" {
		rp.err = d.status.err
	}
	switch {
	case d.err != nil:
		rp.msg, rp.msgBad = d.action+" 失败："+d.err.Error(), true
		c.Toast(rp.msg)
	case strings.TrimSpace(d.out) != "":
		rp.msg, rp.msgBad = d.action+"："+firstLine(d.out), false
		c.Toast("已 " + d.action)
	default:
		rp.msg, rp.msgBad = d.action+" 完成", false
		c.Toast("已 " + d.action)
	}
}

func (g *Git) View(c *ui.Context) {
	t := c.Theme()
	g.svc = c.Services()
	if g.scans == nil {
		g.picks = make(chan gitPicked, 4)
		g.scans = make(chan gitScanDone, 8)
		g.acts = make(chan gitActDone, 16)
	}
	widgets.Drain(g.picks, func(p gitPicked) { g.addRoot(c, p) })
	widgets.Drain(g.scans, g.applyScan)
	widgets.Drain(g.acts, func(d gitActDone) { g.applyAct(c, d) })

	// Roots remembered from the last run are scanned when the page first
	// shows, not when the window is made: the channels their scans report
	// on are made just above.
	for _, r := range g.roots {
		if r.scanned.IsZero() && !r.scanning {
			g.scan(r)
		}
	}

	if len(g.roots) == 0 {
		g.tab = 0
	} else if g.tab >= len(g.roots) {
		g.tab = len(g.roots) - 1
	}
	if g.tab < 0 {
		g.tab = 0
	}

	g.toolbar(c, t)
	if len(g.roots) == 0 {
		ui.Column(c).Grow(1).Center().Gap(8).Children(func() {
			widgets.Hint(c, "还没有 Root 目录。")
			widgets.Hint(c, "选一个目录，它下面所有的 git 代码库都会列在这里。")
		})
		return
	}
	g.tabBar(c, t)
	g.rootView(c, t, g.roots[g.tab])
}

// toolbar is the row above the tabs: adding a root, scanning it again, and
// what the last scan came to.
func (g *Git) toolbar(c *ui.Context, t *ui.Theme) {
	ui.Row(c).Gap(8).AlignItems(ui.Center).Wrap().Children(func() {
		add := ui.PrimaryButton(c.Key("addroot"), "选择 Root 目录…").Disabled(g.picking).
			Tooltip("选一个目录，扫描它下面所有的 git 代码库")
		if add.Clicked() {
			g.pick(c)
		}
		if g.picking {
			widgets.Hint(c, "等待选择…")
		}
		if len(g.roots) == 0 {
			return
		}
		r := g.roots[g.tab]
		rescan := ui.Button(c.Key("rescan"), "重新扫描").Disabled(r.scanning).
			Tooltip("重新扫描 " + r.path)
		if rescan.Clicked() {
			g.scan(r)
		}
		if ui.Button(c.Key("removeroot"), "关闭").Disabled(r.scanning).
			Tooltip("关闭当前 Root 标签，磁盘上的文件不动；下次启动也不再打开").Clicked() {
			g.removeRoot(g.tab)
		}
		ui.Spacer(c)
		widgets.Hint(c, scanHint(r))
		if r.err != "" {
			ui.Text(c, clip("扫描失败："+r.err, 60)).FontSize(widgets.Scaled(12)).TextColor(t.Danger).Tooltip(r.err)
		}
	})
}

// tabBar is one tab per root, named by its directory, each with a × that
// closes it.
func (g *Git) tabBar(c *ui.Context, t *ui.Theme) {
	closed := -1
	tabs := ui.TabsBase(c, &g.tab, len(g.roots))
	tabs.List.Gap(4).Children(func() {
		for i, r := range g.roots {
			on := i == g.tab
			tab := tabs.Tab(i).Padding(3, 8).Radius(6).Tooltip(r.path).Cursor(ui.CursorPointer)
			if on {
				tab.Background(t.Surface).BorderWidth(0, 0, 2, 0).BorderColor(t.Accent)
			} else if tab.Hovered() {
				tab.Background(t.Surface)
			}
			tab.Children(func() {
				col := t.TextMuted
				if on {
					col = t.Text
				}
				ui.Icon(c, icoGitFolder).FontSize(widgets.Scaled(13)).TextColor(col)
				ui.Text(c, leftEllipsis(g.tabLabel(i), 20)).FontSize(widgets.Scaled(12)).TextColor(col).SingleLine()
				if g.closeTab(c, t, i) {
					closed = i
				}
			})
		}
	})
	if closed >= 0 {
		// Closed once every tab is built: the tabs after it in this pass
		// are still where the loop found them.
		g.removeRoot(closed)
	}
}

// closeTab is the × on a tab, named for the root it closes.
func (g *Git) closeTab(c *ui.Context, t *ui.Theme, i int) bool {
	x := ui.ButtonBase(c.Key("closetab")).Size(16, 16).Radius(3).Shrink(0).
		Label("关闭 " + shortRoot(g.roots[i].path)).
		Tooltip("关闭这个 Root，磁盘上的文件不动")
	if x.Hovered() {
		x.Background(t.Border)
	}
	x.Children(func() {
		ui.Icon(c, icoClose).FontSize(widgets.Scaled(10)).TextColor(t.TextMuted)
	})
	return x.Clicked()
}

// tabLabel names a root by its directory, as tabs are named; the whole path
// is in the tooltip of the tab.
func (g *Git) tabLabel(i int) string {
	name := filepath.Base(g.roots[i].path)
	if name == "" || name == string(filepath.Separator) {
		return shortRoot(g.roots[i].path)
	}
	// Two roots may end in the same name: the one there shows the folder
	// above it as well.
	for j, other := range g.roots {
		if j != i && filepath.Base(other.path) == name {
			return filepath.Base(filepath.Dir(g.roots[i].path)) + "/" + name
		}
	}
	return name
}

// rootView is one root: the search, the buttons over the list, and the list
// of folders and repositories.
func (g *Git) rootView(c *ui.Context, t *ui.Theme, r *gitRoot) {
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		box := ui.TextInput(c.Key("q"), &r.q).Placeholder("搜索代码库…").Grow(1).
			Tooltip("按名字、路径或分支过滤（" + widgets.ShortcutHint(0, "F") + " 把键盘放进来）")
		if c.Shortcut(ui.Cmd, ui.KeyF) {
			box.Focus()
		}
		if r.q != "" && c.Shortcut(0, ui.KeyEscape) {
			r.q = ""
		}
		if r.q != "" {
			widgets.Hint(c, fmt.Sprintf("匹配 %d 个", r.matches()))
		}
		ui.Spacer(c)
		if ui.Button(c.Key("expandall"), "展开").Tooltip("展开所有文件夹").
			Disabled(len(r.collapsed) == 0).Clicked() {
			r.collapsed = map[string]bool{}
		}
		if ui.Button(c.Key("foldall"), "折叠").Tooltip("折叠所有文件夹").
			Disabled(len(r.repos) == 0).Clicked() {
			for _, p := range r.groups() {
				r.collapsed[p] = true
			}
		}
		if iconAction(c, "refreshall", "刷新状态（重新读取各仓库的 git status，不重新遍历目录）",
			icoRefresh, !r.scanning && len(r.repos) > 0) {
			g.refreshStatuses(r)
		}
	})

	r.rows = r.buildRows(r.q)
	switch {
	case r.scanning && len(r.repos) == 0:
		ui.Column(c).Grow(1).Center().Children(func() { widgets.Hint(c, "正在扫描…") })
	case len(r.rows) == 0 && r.q != "":
		ui.Column(c).Grow(1).Center().Gap(6).Children(func() {
			widgets.Hint(c, "没有匹配的代码库")
			if ui.Button(c.Key("clearq"), "清空搜索").Clicked() {
				r.q = ""
			}
		})
	case len(r.rows) == 0:
		ui.Column(c).Grow(1).Center().Gap(6).Children(func() {
			widgets.Hint(c, "这个目录下没有找到 git 代码库")
			rescan := ui.Button(c.Key("rescan2"), "重新扫描")
			if rescan.Clicked() {
				g.scan(r)
			}
		})
	default:
		r.list.Key = func(i int) any { return r.rows[i].key() }
		ui.List(c, &r.list, len(r.rows), func(i int) { g.row(c, t, r, i) }).
			Label("代码库").FillWidth().Grow(1)
	}
}

// row builds one line of the list.
func (g *Git) row(c *ui.Context, t *ui.Theme, r *gitRoot, i int) {
	if i < 0 || i >= len(r.rows) {
		return
	}
	row := r.rows[i]
	if row.repo != nil {
		g.repoRow(c, t, r, row.repo, row.depth)
		return
	}
	g.folderRow(c, t, r, row)
}

// folderRow is a folder of repositories: a click folds it or opens it.
func (g *Git) folderRow(c *ui.Context, t *ui.Theme, r *gitRoot, row gitRow) {
	name := filepath.Base(row.group)
	// The room on the right keeps the buttons clear of the scroll bar,
	// which comes and goes over the last few DIPs of the list.
	line := ui.Row(c).Height(widgets.Scaled(gitRowHeight)).FillWidth().AlignItems(ui.Center).
		Gap(widgets.Scaled(5)).Padding(0, widgets.Scaled(14), 0, widgets.Scaled(4)).
		Cursor(ui.CursorPointer).Tooltip(row.group)
	if line.Hovered() {
		line.Background(t.Surface)
	}
	line.Children(func() {
		if row.depth > 0 {
			ui.Box(c).Width(float32(row.depth) * widgets.Scaled(gitIndent)).Shrink(0)
		}
		caret := icoCaretDown
		if row.folded {
			caret = icoCaretRight
		}
		ui.Icon(c, caret).FontSize(widgets.Scaled(11)).TextColor(t.TextMuted)
		ui.Icon(c, icoGitFolder).FontSize(widgets.Scaled(13)).TextColor(t.TextMuted)
		ui.Text(c, name).FontSize(widgets.Scaled(13)).SingleLine()
		widgets.Hint(c, fmt.Sprintf("%d", row.count))
	})
	if line.Clicked() {
		r.collapsed[row.group] = !r.collapsed[row.group]
	}
}

// repoRow is one repository: its name, what it has to say, and the buttons
// that act on it.
func (g *Git) repoRow(c *ui.Context, t *ui.Theme, r *gitRoot, rp *gitRepo, depth int) {
	line := ui.Row(c).Height(widgets.Scaled(gitRowHeight)).FillWidth().AlignItems(ui.Center).
		Gap(widgets.Scaled(6)).Padding(0, widgets.Scaled(14), 0, widgets.Scaled(4)).
		Tooltip(rp.path + " · " + rp.gitStatus.summary())
	if line.Hovered() {
		line.Background(t.Surface)
	}
	line.Children(func() {
		if depth > 0 {
			ui.Box(c).Width(float32(depth) * widgets.Scaled(gitIndent)).Shrink(0)
		}
		ui.Icon(c, icoGitRepo).FontSize(widgets.Scaled(13)).TextColor(t.TextMuted)
		ui.Text(c, rp.name).FontSize(widgets.Scaled(13)).SingleLine().Ellipsis("…").Shrink(1).MinWidth(0)
		ui.Spacer(c)
		g.repoState(c, t, rp)
		g.branchChip(c, t, rp)
		g.repoActions(c, t, r, rp)
	})
}

// repoState is what the repository has to say: a pull or a push running,
// what the last one came to, and what is not committed, not pushed or not
// pulled.
func (g *Git) repoState(c *ui.Context, t *ui.Theme, rp *gitRepo) {
	s := rp.gitStatus
	if rp.op != "" {
		ui.Text(c, rp.op+" 中…").FontSize(widgets.Scaled(11)).TextColor(t.Accent).Shrink(0)
		return
	}
	if rp.msg != "" {
		col := t.TextMuted
		if rp.msgBad {
			col = t.Danger
		}
		ui.Text(c, clip(rp.msg, 46)).FontSize(widgets.Scaled(11)).TextColor(col).Shrink(0).Tooltip(rp.msg)
	}
	if s.err != "" {
		ui.Text(c, "状态未知").FontSize(widgets.Scaled(11)).TextColor(t.Danger).Shrink(0).Tooltip(s.err)
		return
	}
	if s.conflicts > 0 {
		ui.Text(c, fmt.Sprintf("冲突 %d", s.conflicts)).FontSize(widgets.Scaled(11)).TextColor(t.Danger).Shrink(0).
			Tooltip("有文件冲突，先把它们解决掉")
	}
	if n := s.changes(); n > 0 {
		ui.Text(c, fmt.Sprintf("● %d", n)).FontSize(widgets.Scaled(11)).TextColor(t.Warning).Shrink(0).
			Tooltip(s.summary())
	}
	if s.ahead > 0 {
		ui.Text(c, fmt.Sprintf("↑%d", s.ahead)).FontSize(widgets.Scaled(11)).TextColor(t.Accent).Shrink(0).
			Tooltip(fmt.Sprintf("有 %d 个提交还没 push", s.ahead))
	}
	if s.behind > 0 {
		ui.Text(c, fmt.Sprintf("↓%d", s.behind)).FontSize(widgets.Scaled(11)).TextColor(t.Accent).Shrink(0).
			Tooltip(fmt.Sprintf("远端有 %d 个提交还没 pull", s.behind))
	}
}

// branchChip is the branch the repository has checked out.
func (g *Git) branchChip(c *ui.Context, t *ui.Theme, rp *gitRepo) {
	if rp.err != "" {
		// A repository whose status could not be read has no branch to
		// speak of: the row says why instead.
		return
	}
	name := rp.branch
	switch {
	case rp.detached:
		name = "HEAD"
	case name == "":
		name = "?"
	}
	chip := ui.Row(c).Shrink(0).AlignItems(ui.Center).Gap(widgets.Scaled(4)).
		Padding(widgets.Scaled(1), widgets.Scaled(6)).Radius(4).
		Border(1, t.Border).Tooltip(rp.gitStatus.summary())
	chip.Children(func() {
		ui.Icon(c, icoGitRepo).FontSize(widgets.Scaled(10)).TextColor(t.TextMuted)
		ui.Text(c, name).FontSize(widgets.Scaled(11)).TextColor(t.TextMuted).SingleLine()
	})
}

// repoActions is pull, push, and opening the repository. Pull and push are
// offered when there is something to pull or push.
func (g *Git) repoActions(c *ui.Context, t *ui.Theme, r *gitRoot, rp *gitRepo) {
	s := rp.gitStatus
	busy := rp.op != ""
	known := s.err == "" && !busy

	pullOn := known && !s.detached && s.upstream != "" && s.behind > 0
	pullTip := "pull（git pull --ff-only）"
	switch {
	case busy:
		pullTip = "正在 " + rp.op + "…"
	case s.err != "":
		pullTip = "读不到仓库状态"
	case s.detached:
		pullTip = "分离头指针，没有可 pull 的分支"
	case s.upstream == "":
		pullTip = "没有上游分支，先 push 建立跟踪"
	case s.behind == 0:
		pullTip = "已是远端最新，没有可 pull 的提交"
	}
	if iconAction(c, "pull", pullTip, icoPull, pullOn) {
		g.run(r, rp, "pull")
	}

	pushOn := known && !s.detached && (s.ahead > 0 || s.upstream == "")
	pushTip := "push（git push）"
	switch {
	case busy:
		pushTip = "正在 " + rp.op + "…"
	case s.err != "":
		pushTip = "读不到仓库状态"
	case s.detached:
		pushTip = "分离头指针，没有可 push 的分支"
	case s.upstream == "":
		pushTip = "还没有上游分支，push 会建立跟踪"
	case s.ahead == 0:
		pushTip = "没有要 push 的提交"
	}
	if iconAction(c, "push", pushTip, icoPush, pushOn) {
		g.run(r, rp, "push")
	}

	if iconAction(c, "term", "在终端中打开", icoTerminal, !busy) {
		g.open(c, rp, "terminal")
	}
	if iconAction(c, "files", "在文件管理器中打开", icoGitFolder, !busy) {
		g.open(c, rp, "files")
	}
	g.editorActions(c, rp, busy)
	if iconAction(c, "refresh", "刷新状态（git status）", icoRefresh, !busy) {
		g.refresh(r, rp)
	}
}

// editorActions is the button that opens the repository in an editor. With
// more than one editor installed it opens a menu of them, and remembers the
// one chosen for the next run.
func (g *Git) editorActions(c *ui.Context, rp *gitRepo, busy bool) {
	eds := availableEditors()
	cur := g.editor()
	if cur == nil {
		iconButton(c, "editor", "没有找到常用编辑器", icoCode, false)
		return
	}
	tip := "在 " + cur.Name + " 中打开"
	if len(eds) > 1 {
		tip += "（点开可以换编辑器）"
	}
	b := iconButton(c, "editor", tip, icoCode, !busy)
	if busy {
		return
	}
	if len(eds) == 1 {
		if b.Clicked() {
			g.openWith(c, rp, *cur)
		}
		return
	}
	b.Menu(func(m *ui.Menu) {
		for i := range eds {
			ed := eds[i]
			item := m.Item(ed.Name)
			if ed.Name == cur.Name {
				item.Checked(true)
			}
			if item.Chosen() {
				g.useEditor(ed)
				g.openWith(c, rp, ed)
			}
		}
	})
}

// openWith opens a repository in one editor.
func (g *Git) openWith(c *ui.Context, rp *gitRepo, ed gitEditor) {
	if err := ed.open(rp.path); err != nil {
		c.Toast("打开失败：" + err.Error())
		return
	}
	c.Toast("已在 " + ed.Name + " 中打开 " + rp.name)
}

// open opens a repository the way asked, saying so when it could not.
func (g *Git) open(c *ui.Context, rp *gitRepo, how string) {
	if err := openIn(rp.path, how); err != nil {
		c.Toast("打开失败：" + err.Error())
		return
	}
	switch how {
	case "terminal":
		c.Toast("已在终端中打开 " + rp.name)
	case "files":
		c.Toast("已在文件管理器中打开 " + rp.name)
	}
}

// iconButton is a button with only an icon, named for what it does.
func iconButton(c *ui.Context, key any, tip string, ico *ui.SVG, on bool) ui.Element {
	t := c.Theme()
	b := ui.ButtonBase(c.Key(key)).Size(widgets.Scaled(22), widgets.Scaled(22)).Radius(4).
		Label(tip).Tooltip(tip).Disabled(!on)
	if on && b.Hovered() {
		b.Background(t.SurfaceHover)
	}
	b.Children(func() {
		col := t.TextMuted
		if on {
			col = t.Text
		}
		ui.Icon(c, ico).FontSize(widgets.Scaled(14)).TextColor(col)
	})
	return b
}

// iconAction is iconButton, clicked: a disabled button reports no click.
func iconAction(c *ui.Context, key any, tip string, ico *ui.SVG, on bool) bool {
	b := iconButton(c, key, tip, ico, on)
	return on && b.Clicked()
}

// matches is how many repositories fit the search.
func (r *gitRoot) matches() int {
	if strings.TrimSpace(r.q) == "" {
		return len(r.repos)
	}
	q := strings.ToLower(strings.TrimSpace(r.q))
	n := 0
	for _, rp := range r.repos {
		if rp.match(q) {
			n++
		}
	}
	return n
}

// groups are the folders that hold repositories, relative to the root.
func (r *gitRoot) groups() []string {
	set := map[string]bool{}
	for _, rp := range r.repos {
		dir := filepath.Dir(rp.rel)
		for p := dir; p != "" && p != "."; p = filepath.Dir(p) {
			set[p] = true
		}
	}
	out := make([]string, 0, len(set))
	for p := range set {
		out = append(out, p)
	}
	return out
}

// buildRows lays the repositories out under the folders they are in, with
// the folders that were folded away left out. A search opens them all, so
// that what it found shows.
func (r *gitRoot) buildRows(q string) []gitRow {
	q = strings.ToLower(strings.TrimSpace(q))
	byDir := map[string][]*gitRepo{}
	dirs := map[string]bool{}
	counts := map[string]int{}
	for _, rp := range r.repos {
		if !rp.match(q) {
			continue
		}
		dir := filepath.Dir(rp.rel)
		if dir == "." {
			dir = ""
		}
		byDir[dir] = append(byDir[dir], rp)
		for p := dir; p != "" && p != "."; p = filepath.Dir(p) {
			dirs[p] = true
			counts[p]++
		}
	}
	folded := func(g string) bool { return q == "" && r.collapsed[g] }

	rows := make([]gitRow, 0, len(r.repos)+len(dirs))
	for _, rp := range byDir[""] {
		rows = append(rows, gitRow{repo: rp})
	}
	names := make([]string, 0, len(dirs))
	for d := range dirs {
		names = append(names, d)
	}
	sort.Strings(names)
	for _, d := range names {
		// A folder folded away is still a line of its own; only what is
		// under it goes.
		hidden := false
		for p := filepath.Dir(d); p != "" && p != "."; p = filepath.Dir(p) {
			if folded(p) {
				hidden = true
				break
			}
		}
		if hidden {
			continue
		}
		rows = append(rows, gitRow{
			group:  d,
			depth:  strings.Count(d, string(filepath.Separator)),
			count:  counts[d],
			folded: folded(d),
		})
		if folded(d) {
			continue
		}
		depth := strings.Count(d, string(filepath.Separator)) + 1
		for _, rp := range byDir[d] {
			rows = append(rows, gitRow{repo: rp, depth: depth})
		}
	}
	return rows
}

// scanHint says what the last scan found, or that one is under way.
func scanHint(r *gitRoot) string {
	switch {
	case r.scanning && len(r.repos) > 0:
		return "正在刷新状态…"
	case r.scanning:
		return "正在扫描…"
	case r.err != "":
		return "扫描失败"
	case r.scanned.IsZero():
		return ""
	case len(r.repos) == 0:
		return "没有找到代码库"
	}
	took := r.took.Round(100 * time.Millisecond)
	if took <= 0 {
		took = 100 * time.Millisecond
	}
	return fmt.Sprintf("%d 个代码库 · 用时 %s", len(r.repos), took)
}

// shortRoot writes a path the way the shell does, with ~ for the home
// directory.
func shortRoot(p string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return p
	}
	if p == home {
		return "~"
	}
	if strings.HasPrefix(p, home+string(filepath.Separator)) {
		return "~" + p[len(home):]
	}
	return p
}

// leftEllipsis keeps the end of a path, which is what tells one from another.
func leftEllipsis(s string, n int) string {
	r := []rune(s)
	if len(r) <= n || n <= 1 {
		return s
	}
	return "…" + string(r[len(r)-(n-1):])
}
