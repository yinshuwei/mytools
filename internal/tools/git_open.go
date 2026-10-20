package tools

import (
	"errors"
	"os/exec"
	"runtime"
)

// openIn opens dir the way ask says: "terminal", or "files" for the file
// manager. The editor a repository opens in is the one the machine has,
// which the page finds for itself.
func openIn(dir, ask string) error {
	switch ask {
	case "terminal":
		return launchTerminal(dir)
	case "files":
		return launchFiles(dir)
	}
	return errors.New("未知的打开方式：" + ask)
}

// startDetached starts a program and leaves it to itself: the window is
// not this process, and waiting on it would be waiting on the user.
func startDetached(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}

// startTerminal starts a terminal, with dir as its own working directory as
// well as in the arguments: a terminal that reads no option for it still
// starts where it is told.
func startTerminal(name string, args []string, dir string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}

// launchTerminal opens a terminal in dir, whichever terminal the machine
// has.
func launchTerminal(dir string) error {
	switch runtime.GOOS {
	case "darwin":
		return startDetached("open", "-a", "Terminal", dir)
	case "windows":
		// The empty argument is start's title, which cd would otherwise
		// take; the directory is quoted for the cmd that runs cd.
		return startDetached("cmd", "/c", "start", "", "cmd", "/k", `cd /d "`+dir+`"`)
	}
	var lastErr error
	for _, term := range terminalsFor(dir) {
		if _, err := exec.LookPath(term.name); err != nil {
			continue
		}
		if err := startTerminal(term.name, term.args, dir); err != nil {
			lastErr = err
			continue
		}
		return nil
	}
	if lastErr != nil {
		return lastErr
	}
	return errors.New("没有找到终端程序")
}

// terminalSpec is one way to open a terminal in a directory.
type terminalSpec struct {
	name string
	args []string
}

// terminalsFor lists the terminals to try, best first. Each is asked in its
// own words. The concrete programs come before x-terminal-emulator, which
// on Debian is a wrapper that drops the options it does not know: that is
// how --working-directory was lost, and a terminal opened in the home
// directory.
func terminalsFor(dir string) []terminalSpec {
	return []terminalSpec{
		{"gnome-terminal", []string{"--working-directory=" + dir}},
		{"konsole", []string{"--workdir", dir}},
		{"xfce4-terminal", []string{"--working-directory=" + dir}},
		{"mate-terminal", []string{"--working-directory=" + dir}},
		{"tilix", []string{"--working-directory=" + dir}},
		{"terminator", []string{"--working-directory=" + dir}},
		{"lxterminal", []string{"--working-directory=" + dir}},
		{"foot", []string{"--working-directory=" + dir}},
		{"ghostty", []string{"--working-directory=" + dir}},
		{"alacritty", []string{"--working-directory", dir}},
		{"kitty", []string{"--directory", dir}},
		{"wezterm", []string{"start", "--cwd", dir}},
		// No option for it: the shell they start goes into the directory
		// itself.
		{"x-terminal-emulator", shellIn(dir)},
		{"xterm", shellIn(dir)},
	}
}

// shellIn is the command of a terminal that has no option for the
// directory: a shell that goes there and becomes the user's own.
func shellIn(dir string) []string {
	return []string{"-e", "sh", "-c", `cd "$1" && exec "${SHELL:-sh}"`, "sh", dir}
}

// launchFiles opens the file manager at dir.
func launchFiles(dir string) error {
	switch runtime.GOOS {
	case "darwin":
		return startDetached("open", dir)
	case "windows":
		return startDetached("explorer", dir)
	}
	return startDetached("xdg-open", dir)
}
