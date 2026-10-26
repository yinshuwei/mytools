package shots

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/transfer"
)

// CopyPNG puts a PNG on the clipboard, ready to paste into a chat, a
// document or a picture editor.
func CopyPNG(data []byte) error {
	return mygo.Clipboard.Write(transfer.New(transfer.NewItem(transfer.Bytes(transfer.PNG, data))))
}

// SavePNG asks where to put a PNG and writes it there, returning where. A
// dialog the user closed writes nothing, and is not an error.
func SavePNG(win *mygo.Window, data []byte) (string, error) {
	path, err := mygo.Dialog.Save(mygo.SaveDialogOptions{
		Parent:      win,
		Title:       "保存截图",
		DefaultPath: savePath(time.Now()),
		Filters:     []mygo.FileFilter{{Name: "PNG 图片", Extensions: []string{"png"}}},
	})
	if err != nil || path == "" {
		return "", err
	}
	if !strings.HasSuffix(strings.ToLower(path), ".png") {
		path += ".png"
	}
	return path, os.WriteFile(path, data, 0o644)
}

// saveName is the name the save dialog starts with.
func saveName(t time.Time) string {
	return "screenshot-" + t.Format("20060102-150405") + ".png"
}

// savePath is the file the save dialog starts at: the pictures folder when
// the desktop has one, the home folder otherwise.
func savePath(now time.Time) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return saveName(now)
	}
	dir := filepath.Join(home, "Pictures")
	if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
		dir = home
	}
	return filepath.Join(dir, saveName(now))
}
