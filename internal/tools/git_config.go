package tools

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// gitConfig is what the Git page remembers between runs: the roots it had
// open, and the editor it opened them in.
type gitConfig struct {
	Roots  []string `json:"roots"`
	Editor string   `json:"editor,omitempty"`
}

// gitConfigFile is where this is kept, in the user's configuration
// directory. It is a variable so that tests keep their settings to
// themselves.
var gitConfigFile = defaultGitConfigFile()

// defaultGitConfigFile is the file the settings are kept in: beside the
// other settings of the toolbox, in the desktop's configuration directory.
func defaultGitConfigFile() string {
	dir, err := os.UserConfigDir()
	if err != nil || dir == "" {
		return ""
	}
	return filepath.Join(dir, "mytools", "git-roots.json")
}

// loadConfig reads the roots and the editor of the last run. A root that is
// no longer a directory, and a file that does not read, are left out rather
// than reported: the page is usable without either.
func (g *Git) loadConfig() {
	if gitConfigFile == "" || len(g.roots) > 0 {
		return
	}
	data, err := os.ReadFile(gitConfigFile)
	if err != nil {
		return
	}
	var cfg gitConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return
	}
	g.editorName = cfg.Editor
	for _, p := range cfg.Roots {
		if p == "" {
			continue
		}
		p = filepath.Clean(p)
		info, err := os.Stat(p)
		if err != nil || !info.IsDir() || g.root(p) != nil {
			continue
		}
		g.roots = append(g.roots, &gitRoot{path: p, collapsed: map[string]bool{}})
	}
}

// saveConfig writes the roots and the editor now in use, for the next run.
// It is written beside the file and renamed into place, so that a crash
// while writing cannot leave a half file the next run cannot read.
func (g *Git) saveConfig() {
	if gitConfigFile == "" {
		return
	}
	cfg := gitConfig{Roots: make([]string, 0, len(g.roots)), Editor: g.editorName}
	for _, r := range g.roots {
		cfg.Roots = append(cfg.Roots, r.path)
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(gitConfigFile), 0o755); err != nil {
		return
	}
	tmp := gitConfigFile + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o644); err != nil {
		return
	}
	_ = os.Rename(tmp, gitConfigFile)
}
