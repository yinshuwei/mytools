package tools

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// rootsToTemp keeps the roots a test saves to itself, and puts the real
// file back afterwards.
func rootsToTemp(t *testing.T) {
	t.Helper()
	old := gitConfigFile
	gitConfigFile = filepath.Join(t.TempDir(), "git-roots.json")
	t.Cleanup(func() { gitConfigFile = old })
}

// TestParseGitStatus reads what porcelain v2 says: the branch, what is
// ahead and behind, and what is changed.
func TestParseGitStatus(t *testing.T) {
	const out = `# branch.oid 0123456789abcdef
# branch.head main
# branch.upstream origin/main
# branch.ab +2 -3
1 .M N... 100644 100644 100644 abc def a.txt
1 M. N... 100644 100644 100644 abc def b.txt
2 R. N... 100644 100644 100644 abc def c.txt
u UU N... 100644 100644 100644 abc def d.txt
? new.txt
`
	s := parseGitStatus(out)
	if s.branch != "main" || s.detached {
		t.Errorf("分支 = %q，分离 = %v", s.branch, s.detached)
	}
	if s.upstream != "origin/main" {
		t.Errorf("上游 = %q", s.upstream)
	}
	if s.ahead != 2 || s.behind != 3 {
		t.Errorf("领先/落后 = %d/%d，want 2/3", s.ahead, s.behind)
	}
	if s.staged != 2 {
		t.Errorf("已暂存 = %d，want 2", s.staged)
	}
	if s.unstaged != 1 {
		t.Errorf("未暂存 = %d，want 1", s.unstaged)
	}
	if s.untracked != 1 {
		t.Errorf("未跟踪 = %d，want 1", s.untracked)
	}
	if s.conflicts != 1 {
		t.Errorf("冲突 = %d，want 1", s.conflicts)
	}
	if s.clean() {
		t.Error("有改动却说是干净的")
	}
}

// TestParseGitStatusDetached checks a HEAD that is not on a branch, and a
// repository with nothing to say.
func TestParseGitStatusDetached(t *testing.T) {
	s := parseGitStatus("# branch.oid 0123456789abcdef\n# branch.head (detached)\n")
	if !s.detached || s.branch != "" {
		t.Errorf("分离 = %v，分支 = %q", s.detached, s.branch)
	}
	if !s.clean() {
		t.Errorf("没有改动却说 %q", s.summary())
	}
	if got := s.summary(); got == "" {
		t.Error("没有给出说明")
	}
}

// TestFindGitRepos walks a tree: a clone's .git directory and a worktree's
// .git file both count, and hidden directories and installed packages are
// left alone.
func TestFindGitRepos(t *testing.T) {
	root := t.TempDir()
	mkdir := func(rel string) string {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatal(err)
		}
		return p
	}
	mk := func(rel string) {
		p := mkdir(rel)
		if err := os.MkdirAll(filepath.Join(p, ".git"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	mk("alpha")
	mk("group/beta")
	mk("group/deep/gamma")
	// A worktree records its .git as a file.
	wt := mkdir("group/worktree")
	if err := os.WriteFile(filepath.Join(wt, ".git"), []byte("gitdir: /elsewhere\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mk("node_modules/skip")
	mk(".hidden/skip")
	if err := os.MkdirAll(filepath.Join(root, "plain"), 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := findGitRepos(root)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		filepath.Join(root, "alpha"),
		filepath.Join(root, "group/beta"),
		filepath.Join(root, "group/deep/gamma"),
		filepath.Join(root, "group/worktree"),
	}
	if len(got) != len(want) {
		t.Fatalf("找到 %v，want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("第 %d 个 = %q，want %q", i, got[i], want[i])
		}
	}
}

// TestGitRows groups repositories under their folders, folds them away, and
// opens them again for a search.
func TestGitRows(t *testing.T) {
	r := &gitRoot{
		path:      "/workspace",
		collapsed: map[string]bool{},
		repos: []*gitRepo{
			{rel: "a", name: "a"},
			{rel: "g/b", name: "b"},
			{rel: "g/h/c", name: "c"},
		},
	}
	rows := r.buildRows("")
	if len(rows) != 5 {
		t.Fatalf("行数 = %d，want 5", len(rows))
	}
	// The repository at the root, then the folder g, its repository, the
	// folder g/h, and the repository under that.
	if rows[0].repo == nil || rows[0].repo.name != "a" {
		t.Errorf("第 0 行 = %+v，want 仓库 a", rows[0])
	}
	if rows[1].repo != nil || rows[1].group != "g" || rows[1].depth != 0 || rows[1].count != 2 {
		t.Errorf("第 1 行 = %+v，want 文件夹 g，含 2 个", rows[1])
	}
	if rows[2].repo == nil || rows[2].repo.name != "b" || rows[2].depth != 1 {
		t.Errorf("第 2 行 = %+v，want 仓库 b，缩进 1", rows[2])
	}
	if rows[3].group != "g/h" || rows[3].depth != 1 {
		t.Errorf("第 3 行 = %+v，want 文件夹 g/h", rows[3])
	}
	if rows[4].repo == nil || rows[4].repo.name != "c" || rows[4].depth != 2 {
		t.Errorf("第 4 行 = %+v，want 仓库 c，缩进 2", rows[4])
	}

	// Folding g hides what is under it, its nested folder included.
	r.collapsed["g"] = true
	rows = r.buildRows("")
	if len(rows) != 2 || rows[1].group != "g" || !rows[1].folded {
		t.Fatalf("折叠 g 后 = %+v，want a 与折叠的 g", rows)
	}

	// A search opens the folders: what it found must show.
	rows = r.buildRows("c")
	if len(rows) != 3 {
		t.Fatalf("搜索 c 后 = %+v，want g、g/h、c", rows)
	}
	if rows[0].group != "g" || rows[1].group != "g/h" || rows[2].repo == nil || rows[2].repo.name != "c" {
		t.Errorf("搜索 c 后 = %+v", rows)
	}

	// Folding works with the keyboard-free button as well: everything.
	for _, p := range r.groups() {
		r.collapsed[p] = true
	}
	if rows = r.buildRows(""); len(rows) != 2 {
		t.Errorf("全部折叠后 = %+v，want a 与 g", rows)
	}
}

// TestGitStatusRefresh checks that a refresh reads the statuses again in
// place: the rows keep their place, and what a repository is doing stays.
func TestGitStatusRefresh(t *testing.T) {
	rp := &gitRepo{path: "/w/a", rel: "a", name: "a", op: "pull", msg: "上一次"}
	r := &gitRoot{path: "/w", repos: []*gitRepo{rp}}
	g := &Git{roots: []*gitRoot{r}}
	g.applyScan(gitScanDone{
		root: "/w",
		statuses: map[string]gitStatus{
			"/w/a": {branch: "dev", upstream: "origin/dev", ahead: 2},
		},
	})
	if rp.branch != "dev" || rp.ahead != 2 {
		t.Errorf("刷新后 = %+v", rp.gitStatus)
	}
	if rp.op != "pull" || rp.msg != "上一次" {
		t.Errorf("刷新动了正在进行的动作：op=%q msg=%q", rp.op, rp.msg)
	}
	if len(r.repos) != 1 || r.repos[0] != rp {
		t.Errorf("刷新换掉了仓库行：%+v", r.repos)
	}
	if r.scanned.IsZero() {
		t.Error("刷新后没有记下时间")
	}
}

// TestGitRootsRemembered checks that the roots open now are the roots of
// the next run.
func TestGitRootsRemembered(t *testing.T) {
	rootsToTemp(t)
	dir := t.TempDir()
	g := &Git{roots: []*gitRoot{{path: dir, collapsed: map[string]bool{}}}}
	g.saveConfig()

	next := &Git{}
	next.loadConfig()
	if len(next.roots) != 1 || next.roots[0].path != dir {
		t.Fatalf("读回来的 roots = %+v", next.roots)
	}
	if next.roots[0].collapsed == nil {
		t.Error("折叠状态没有初始化")
	}
	// Reading again adds nothing, as the window is given once.
	next.loadConfig()
	if len(next.roots) != 1 {
		t.Errorf("再读一次 = %d 个 root", len(next.roots))
	}
}

// TestGitRootsGoneDropped checks that a root no longer on the disk does not
// come back as an empty tab.
func TestGitRootsGoneDropped(t *testing.T) {
	rootsToTemp(t)
	dir := t.TempDir()
	gone := filepath.Join(dir, "gone")
	data, err := json.Marshal(gitConfig{Roots: []string{gone, dir}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(gitConfigFile, data, 0o644); err != nil {
		t.Fatal(err)
	}
	g := &Git{}
	g.loadConfig()
	if len(g.roots) != 1 || g.roots[0].path != dir {
		t.Fatalf("读回来的 roots = %+v，want 只剩 %q", g.roots, dir)
	}
}

// TestGitEditor checks that the button names the editor the machine has,
// that a machine with more than one opens a menu of them, and that the one
// chosen is remembered.
func TestGitEditor(t *testing.T) {
	rootsToTemp(t)
	old := availableEditors
	defer func() { availableEditors = old }()

	var opened string
	availableEditors = func() []gitEditor {
		return []gitEditor{
			{Name: "VS Code", open: func(dir string) error { opened = "code:" + dir; return nil }},
			{Name: "VSCodium", open: func(dir string) error { opened = "codium:" + dir; return nil }},
		}
	}

	g := &Git{}
	if ed := g.editor(); ed == nil || ed.Name != "VS Code" {
		t.Fatalf("默认编辑器 = %+v，want VS Code", ed)
	}

	// A repository of its own, so that the row and its buttons show.
	dir := t.TempDir()
	repo := filepath.Join(dir, "repo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	g.roots = []*gitRoot{{
		path: dir, collapsed: map[string]bool{}, scanned: time.Now(),
		repos: []*gitRepo{{path: repo, rel: "repo", name: "repo"}},
	}}
	tr := ui.NewTester(g.View, 900, 600)
	tr.Frame()
	// The button names the first editor; with two installed it opens a menu.
	if err := tr.Click("在 VS Code 中打开（点开可以换编辑器）"); err != nil {
		t.Fatal(err)
	}
	menu := tr.Menu()
	if len(menu) != 2 || menu[0] != "VS Code" || menu[1] != "VSCodium" {
		t.Fatalf("菜单 = %v，want [VS Code VSCodium]", menu)
	}
	if err := tr.ChooseMenuItem("VSCodium"); err != nil {
		t.Fatalf("菜单里没有 VSCodium：%v", err)
	}
	if opened != "codium:"+repo {
		t.Errorf("打开的是 %q，want codium:%s", opened, repo)
	}
	if g.editorName != "VSCodium" {
		t.Errorf("记住的编辑器 = %q，want VSCodium", g.editorName)
	}
	next := &Git{}
	next.loadConfig()
	if next.editorName != "VSCodium" {
		t.Errorf("下次启动的编辑器 = %q，want VSCodium", next.editorName)
	}

	// A machine without one has no editor to name.
	availableEditors = func() []gitEditor { return nil }
	if ed := (&Git{}).editor(); ed != nil {
		t.Errorf("没有编辑器却选出了 %q", ed.Name)
	}
}

// TestGitCloseTab checks the × on a tab: the root goes, the tab shown stays
// the same root, and the next run remembers what is left.
func TestGitCloseTab(t *testing.T) {
	rootsToTemp(t)
	a, b := t.TempDir(), t.TempDir()
	g := &Git{
		roots: []*gitRoot{
			{path: a, collapsed: map[string]bool{}},
			{path: b, collapsed: map[string]bool{}},
		},
		tab: 1,
	}
	tr := ui.NewTester(g.View, 900, 600)
	tr.Frame()
	if err := tr.Click("关闭 " + shortRoot(a)); err != nil {
		t.Fatal(err)
	}
	if len(g.roots) != 1 || g.roots[0].path != b {
		t.Fatalf("关掉 a 后 roots = %+v", g.roots)
	}
	if g.tab != 0 {
		t.Errorf("当前标签 = %d，want 0（还是 b）", g.tab)
	}
	next := &Git{}
	next.loadConfig()
	if len(next.roots) != 1 || next.roots[0].path != b {
		t.Errorf("下次启动的 roots = %+v，want [%q]", next.roots, b)
	}
}

// gitIn runs git in dir, failing the test when it does not work.
func gitIn(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// TestGitFetchUpdatesStatus checks that a refresh counts what the server
// has now: a clone's status stays as of its last fetch until one is made,
// and is behind the server after it.
func TestGitFetchUpdatesStatus(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("没有 git")
	}
	dir := t.TempDir()
	bare, work, other := filepath.Join(dir, "bare.git"), filepath.Join(dir, "work"), filepath.Join(dir, "other")
	if err := os.MkdirAll(bare, 0o755); err != nil {
		t.Fatal(err)
	}
	gitIn(t, bare, "init", "--bare", "-b", "main")
	gitIn(t, dir, "clone", bare, work)
	config := func(p string) {
		gitIn(t, p, "config", "user.email", "a@b.c")
		gitIn(t, p, "config", "user.name", "a")
	}
	write := func(p, s string) {
		if err := os.WriteFile(filepath.Join(p, "f.txt"), []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	config(work)
	write(work, "one")
	gitIn(t, work, "add", ".")
	gitIn(t, work, "commit", "-qm", "one")
	gitIn(t, work, "push", "-qu", "origin", "main")

	gitIn(t, dir, "clone", bare, other)
	config(other)
	write(other, "two")
	gitIn(t, other, "commit", "-qam", "two")
	gitIn(t, other, "push", "-q")

	if st := readGitStatus(work); st.behind != 0 {
		t.Fatalf("没有 fetch 就落后 %d：状态不该自己去问服务器", st.behind)
	}
	if err := runGitFetch(work); err != nil {
		t.Fatal(err)
	}
	if st := readGitStatus(work); st.behind != 1 {
		t.Errorf("fetch 后 behind = %d，want 1", st.behind)
	}
}
