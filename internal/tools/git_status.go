package tools

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	// gitMaxDepth is how deep under a root a repository may be found. The
	// walk is over whatever the user points it at, home directories
	// included, and past this a tree is another project's business.
	gitMaxDepth = 8
	// gitStatusWorkers is how many git processes read statuses at once: one
	// each is a process, and a root may hold hundreds of repositories.
	gitStatusWorkers = 12
	// gitStatusTimeout bounds one status read, which is a local operation.
	gitStatusTimeout = 20 * time.Second
	// gitActionTimeout bounds a pull or a push, which talks to a remote.
	gitActionTimeout = 5 * time.Minute
)

// gitSkipDirs are the directories the walk does not go into: the contents
// of installed packages hold no repository of the user's, and are many.
// Hidden directories are skipped as well.
var gitSkipDirs = map[string]bool{
	"node_modules": true,
	"vendor":       true,
	"Pods":         true,
	"Library":      true,
}

// gitStatus is what a repository says about itself, read from
// git status --porcelain=v2 --branch.
type gitStatus struct {
	branch    string // the branch checked out, when not detached
	detached  bool   // HEAD is not on a branch
	upstream  string // the remote branch it tracks, if any
	ahead     int    // commits made here and not pushed
	behind    int    // commits on the remote and not here
	staged    int    // tracked files with changes in the index
	unstaged  int    // tracked files with changes in the working tree
	untracked int    // files git does not know
	conflicts int    // files with merge conflicts
	err       string // why the status could not be read
}

// changes is how many files are not committed.
func (s gitStatus) changes() int { return s.staged + s.unstaged + s.untracked }

// clean says there is nothing to commit, pull or push.
func (s gitStatus) clean() bool {
	return s.err == "" && s.changes() == 0 && s.conflicts == 0 &&
		s.ahead == 0 && s.behind == 0
}

// summary tells the state in a line, for a tooltip or a toast.
func (s gitStatus) summary() string {
	if s.err != "" {
		return "读不到状态：" + s.err
	}
	var parts []string
	switch {
	case s.detached:
		parts = append(parts, "分离头指针")
	case s.branch != "":
		parts = append(parts, "分支 "+s.branch)
	}
	if s.upstream != "" {
		parts = append(parts, "上游 "+s.upstream)
	} else if !s.detached {
		parts = append(parts, "没有上游分支")
	}
	if s.ahead > 0 {
		parts = append(parts, fmt.Sprintf("领先 %d 个提交", s.ahead))
	}
	if s.behind > 0 {
		parts = append(parts, fmt.Sprintf("落后 %d 个提交", s.behind))
	}
	if s.staged > 0 {
		parts = append(parts, fmt.Sprintf("已暂存 %d", s.staged))
	}
	if s.unstaged > 0 {
		parts = append(parts, fmt.Sprintf("未暂存 %d", s.unstaged))
	}
	if s.untracked > 0 {
		parts = append(parts, fmt.Sprintf("未跟踪 %d", s.untracked))
	}
	if s.conflicts > 0 {
		parts = append(parts, fmt.Sprintf("冲突 %d", s.conflicts))
	}
	if len(parts) == 0 {
		return "工作区干净"
	}
	return strings.Join(parts, " · ")
}

// gitRepo is one repository found under a root.
type gitRepo struct {
	path string // where it is
	rel  string // its directory relative to the root, "." for the root itself
	name string // the directory's name

	gitStatus

	op     string // the pull or push running now, if any
	msg    string // what the last one came to
	msgBad bool   // it failed
}

// match reports whether the repository fits the search q, already lowered
// and trimmed.
func (r *gitRepo) match(q string) bool {
	if q == "" {
		return true
	}
	for _, s := range []string{r.rel, r.name, r.branch} {
		if strings.Contains(strings.ToLower(s), q) {
			return true
		}
	}
	return false
}

// newGitRepo is one repository found under root, with its status.
func newGitRepo(root, path string, st gitStatus) *gitRepo {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		rel = path
	}
	name := filepath.Base(path)
	if rel != "." {
		name = filepath.Base(rel)
	}
	return &gitRepo{path: path, rel: rel, name: name, gitStatus: st}
}

// scanGitRoots finds every repository under root and reads their statuses.
func scanGitRoots(root string) ([]*gitRepo, error) {
	if _, err := exec.LookPath("git"); err != nil {
		return nil, errors.New("没有找到 git，请先安装并把它放进 PATH")
	}
	paths, err := findGitRepos(root)
	if err != nil {
		return nil, err
	}
	statuses := readStatuses(paths)
	repos := make([]*gitRepo, len(paths))
	for i, p := range paths {
		repos[i] = newGitRepo(root, p, statuses[i])
	}
	sort.Slice(repos, func(i, j int) bool { return repos[i].rel < repos[j].rel })
	return repos, nil
}

// readStatuses reads the status of each path, a few at a time, as each is a
// process of its own.
func readStatuses(paths []string) []gitStatus {
	statuses := make([]gitStatus, len(paths))
	sem := make(chan struct{}, gitStatusWorkers)
	var wg sync.WaitGroup
	for i, p := range paths {
		wg.Add(1)
		go func(i int, p string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			statuses[i] = readGitStatus(p)
		}(i, p)
	}
	wg.Wait()
	return statuses
}

// findGitRepos walks root for directories holding a .git: a directory in a
// usual clone, a file in a worktree or a submodule.
func findGitRepos(root string) ([]string, error) {
	root = filepath.Clean(root)
	var paths []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if p == root {
				return err
			}
			// A directory the walk may not read is skipped, not fatal:
			// one unreadable corner of a home directory must not cost the
			// whole scan.
			return nil
		}
		if !d.IsDir() {
			return nil
		}
		if p != root {
			name := d.Name()
			if name == ".git" || strings.HasPrefix(name, ".") || gitSkipDirs[name] {
				return filepath.SkipDir
			}
			if gitRelDepth(root, p) >= gitMaxDepth {
				return filepath.SkipDir
			}
		}
		if isGitRepo(p) {
			paths = append(paths, p)
		}
		return nil
	})
	sort.Strings(paths)
	return paths, err
}

// isGitRepo says whether dir holds a repository.
func isGitRepo(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, ".git"))
	return err == nil
}

// gitRelDepth is how many directories deep p is under root.
func gitRelDepth(root, p string) int {
	rel, err := filepath.Rel(root, p)
	if err != nil || rel == "." {
		return 0
	}
	return strings.Count(rel, string(filepath.Separator)) + 1
}

// readGitStatus reads the branch, what is ahead and behind, and what is
// changed, in one git call.
func readGitStatus(dir string) gitStatus {
	out, err := gitOutput(dir, gitStatusTimeout, "status", "--porcelain=v2", "--branch")
	if err != nil {
		return gitStatus{err: err.Error()}
	}
	return parseGitStatus(out)
}

// parseGitStatus reads porcelain v2 with --branch.
func parseGitStatus(out string) gitStatus {
	var s gitStatus
	for _, line := range strings.Split(out, "\n") {
		switch {
		case strings.HasPrefix(line, "# branch.head "):
			s.branch = strings.TrimSpace(strings.TrimPrefix(line, "# branch.head "))
			if s.branch == "(detached)" {
				s.detached, s.branch = true, ""
			}
		case strings.HasPrefix(line, "# branch.upstream "):
			s.upstream = strings.TrimSpace(strings.TrimPrefix(line, "# branch.upstream "))
		case strings.HasPrefix(line, "# branch.ab "):
			fmt.Sscanf(strings.TrimPrefix(line, "# branch.ab "), "+%d -%d", &s.ahead, &s.behind)
		case strings.HasPrefix(line, "1 "), strings.HasPrefix(line, "2 "):
			// A changed or renamed entry: XY are the index and working
			// tree states.
			if f := strings.Fields(line); len(f) >= 2 {
				s.countXY(f[1])
			}
		case strings.HasPrefix(line, "u "):
			s.conflicts++
		case strings.HasPrefix(line, "? "):
			s.untracked++
		}
	}
	return s
}

// countXY counts a porcelain v2 XY pair, where "." is no change.
func (s *gitStatus) countXY(xy string) {
	if len(xy) < 2 {
		return
	}
	if xy[0] != '.' {
		s.staged++
	}
	if xy[1] != '.' {
		s.unstaged++
	}
}

// runGitAction pulls or pushes one repository. A branch without an upstream
// is pushed with one set, so that the first push of a new branch works.
func runGitAction(dir, action, branch, upstream string) (string, error) {
	switch action {
	case "pull":
		// --ff-only keeps a pull from making a merge commit, which is a
		// decision for the user and would want an editor.
		return gitOutput(dir, gitActionTimeout, "pull", "--ff-only")
	case "push":
		if upstream == "" && branch != "" {
			return gitOutput(dir, gitActionTimeout, "push", "--set-upstream", "origin", branch)
		}
		return gitOutput(dir, gitActionTimeout, "push")
	}
	return "", errors.New("未知的动作：" + action)
}

// gitOutput runs git in dir and returns what it wrote, error and all.
// Prompts are off: a credential prompt in a window with no terminal would
// wait forever, and a failure tells what is wrong.
func gitOutput(dir string, timeout time.Duration, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(),
		"GIT_TERMINAL_PROMPT=0",
		"GIT_OPTIONAL_LOCKS=0",
		"GIT_EDITOR=true",
		"LC_ALL=C",
	)
	var buf bytes.Buffer
	cmd.Stdout, cmd.Stderr = &buf, &buf
	err := cmd.Run()
	out := buf.String()
	if ctx.Err() != nil {
		return out, fmt.Errorf("git %s 超时", args[0])
	}
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return out, errors.New("没有找到 git")
		}
		if msg := firstLine(out); msg != "" {
			return out, errors.New(msg)
		}
		return out, err
	}
	return out, nil
}

// firstLine is the first line of what git wrote, cut to what a toast or a
// tooltip shows.
func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	return clip(strings.TrimSpace(s), 160)
}

// clip cuts s to n characters, ending it with an ellipsis.
func clip(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	if n <= 1 {
		return string(r[:n])
	}
	return string(r[:n-1]) + "…"
}
