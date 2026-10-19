package tools

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
)

// gitEditor is an editor the machine has, and how a directory is opened in
// it.
type gitEditor struct {
	Name string
	open func(dir string) error
}

// editorCandidate is one editor worth looking for: its name as the button
// says it, the command it puts on the PATH, the bundle it installs on a Mac,
// and the program a Windows install leaves behind.
type editorCandidate struct {
	name string
	bin  string
	app  string
	win  string
}

// editorCandidates are the editors looked for, in the order they are
// preferred: the first one found is the one the button names, until another
// is chosen from its menu. A machine with more than one takes the first
// here, so put the one to prefer first.
var editorCandidates = []editorCandidate{
	{"VS Code", "code", "Visual Studio Code.app", `%LOCALAPPDATA%\Programs\Microsoft VS Code\bin\code.cmd`},
	{"VS Code Insiders", "code-insiders", "Visual Studio Code - Insiders.app", `%LOCALAPPDATA%\Programs\Microsoft VS Code Insiders\bin\code-insiders.cmd`},
	{"VSCodium", "codium", "VSCodium.app", `%LOCALAPPDATA%\Programs\VSCodium\bin\codium.cmd`},
	{"Cursor", "cursor", "Cursor.app", `%LOCALAPPDATA%\Programs\cursor\Cursor.exe`},
	{"Windsurf", "windsurf", "Windsurf.app", `%LOCALAPPDATA%\Programs\Windsurf\Windsurf.exe`},
	{"Zed", "zed", "Zed.app", ""},
	{"Sublime Text", "subl", "Sublime Text.app", ""},
	{"IntelliJ IDEA", "idea", "IntelliJ IDEA.app", ""},
	{"GoLand", "goland", "GoLand.app", ""},
	{"WebStorm", "webstorm", "WebStorm.app", ""},
	{"PyCharm", "pycharm", "PyCharm.app", ""},
	{"PhpStorm", "phpstorm", "PhpStorm.app", ""},
	{"Android Studio", "studio", "Android Studio.app", ""},
	{"Nova", "nova", "Nova.app", ""},
	{"Kate", "kate", "", ""},
	{"GNOME Text Editor", "gnome-text-editor", "", ""},
	{"gedit", "gedit", "", ""},
}

var (
	editorsOnce sync.Once
	editorsList []gitEditor
)

// availableEditors is the editors the machine has, best first. It is a
// variable so that a test can put a set there.
var availableEditors = findEditors

// findEditors looks for the editors once, when the page first needs one.
func findEditors() []gitEditor {
	editorsOnce.Do(func() { editorsList = detectEditors() })
	return editorsList
}

// detectEditors lists the editors the machine has, best first.
func detectEditors() []gitEditor {
	var out []gitEditor
	for _, cand := range editorCandidates {
		if ed, ok := detectEditor(cand); ok {
			out = append(out, ed)
		}
	}
	return out
}

// detectEditor finds one editor: on the PATH, as a Mac application, or
// where its Windows installer puts it.
func detectEditor(cand editorCandidate) (gitEditor, bool) {
	if p, err := exec.LookPath(cand.bin); err == nil {
		return gitEditor{Name: cand.name, open: func(dir string) error {
			return startDetached(p, dir)
		}}, true
	}
	switch runtime.GOOS {
	case "darwin":
		if cand.app == "" {
			return gitEditor{}, false
		}
		home, _ := os.UserHomeDir()
		for _, dir := range []string{"/Applications", filepath.Join(home, "Applications")} {
			if _, err := os.Stat(filepath.Join(dir, cand.app)); err != nil {
				continue
			}
			app := cand.app
			return gitEditor{Name: cand.name, open: func(dir string) error {
				return startDetached("open", "-a", app, dir)
			}}, true
		}
	case "windows":
		if cand.win == "" {
			return gitEditor{}, false
		}
		if p := os.ExpandEnv(cand.win); gitFileExists(p) {
			return gitEditor{Name: cand.name, open: func(dir string) error {
				return startDetached(p, dir)
			}}, true
		}
	}
	return gitEditor{}, false
}

// gitFileExists says whether p is a file, as opposed to a directory or
// nothing at all.
func gitFileExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
}

// editor is the one this page opens a repository in: the one chosen last
// run when the machine still has it, and the first it has otherwise.
func (g *Git) editor() *gitEditor {
	eds := availableEditors()
	if len(eds) == 0 {
		return nil
	}
	for i := range eds {
		if eds[i].Name == g.editorName {
			return &eds[i]
		}
	}
	return &eds[0]
}

// useEditor remembers an editor for the next run.
func (g *Git) useEditor(ed gitEditor) {
	if g.editorName == ed.Name {
		return
	}
	g.editorName = ed.Name
	g.saveConfig()
}
