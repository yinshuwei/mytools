package tools

import (
	"fmt"
	"os"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// pick is what a background file dialog sends back: what it chose, read, or
// why it could not.
type pick struct {
	path string
	data []byte
	err  error
}

// pickFile asks for a file in the background, so that the window keeps
// drawing, and delivers it on ch once it is read; the frame that reads ch
// takes it. An empty path is a dialog the user closed.
func pickFile(win *mygo.Window, svc ui.Services, title string, filters []mygo.FileFilter, ch chan<- pick) {
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Parent:  win,
			Title:   title,
			Filters: filters,
		})
		switch {
		case err != nil:
			ch <- pick{err: err}
		case len(paths) == 0:
			ch <- pick{}
		default:
			data, err := os.ReadFile(paths[0])
			ch <- pick{path: paths[0], data: data, err: err}
		}
		svc.Invalidate()
	}()
}

// imageFilters limits a file dialog to images.
var imageFilters = []mygo.FileFilter{{
	Name:       "图片",
	Extensions: []string{"png", "jpg", "jpeg", "gif", "webp", "bmp"},
}}

// humanBytes formats a size for the eye.
func humanBytes(n int) string {
	switch {
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(n)/(1<<10))
	}
	return fmt.Sprintf("%d B", n)
}
