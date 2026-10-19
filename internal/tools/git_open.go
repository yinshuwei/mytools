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
	terminals := []struct {
		name string
		args []string
	}{
		{"x-terminal-emulator", []string{"--working-directory=" + dir}},
		{"gnome-terminal", []string{"--working-directory=" + dir}},
		{"konsole", []string{"--workdir", dir}},
		{"xfce4-terminal", []string{"--working-directory=" + dir}},
		{"alacritty", []string{"--working-directory", dir}},
		{"kitty", []string{"--directory", dir}},
		{"wezterm", []string{"start", "--cwd", dir}},
	}
	for _, term := range terminals {
		if _, err := exec.LookPath(term.name); err != nil {
			continue
		}
		if err := startDetached(term.name, term.args...); err != nil {
			return err
		}
		return nil
	}
	return errors.New("没有找到终端程序")
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
