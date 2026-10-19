package app

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
)

const (
	// minZoom and maxZoom bound the size the pages may be drawn at, and
	// zoomStep is what the slider snaps to.
	minZoom  = 0.8
	maxZoom  = 1.5
	zoomStep = 0.05
)

// settings is what the user chose about the window's look, kept between
// runs.
type settings struct {
	Theme string  `json:"theme"` // "light", "dark", or "system"
	Zoom  float64 `json:"zoom"`  // 1 is the size the desktop asks for
}

// settingsFile is where they are kept, beside the other settings of the
// toolbox. It is a variable so that tests keep their settings to themselves.
var settingsFile = defaultSettingsFile()

// defaultSettingsFile is the file the appearance is kept in, in the user's
// configuration directory.
func defaultSettingsFile() string {
	dir, err := os.UserConfigDir()
	if err != nil || dir == "" {
		return ""
	}
	return filepath.Join(dir, "mytools", "settings.json")
}

// loadSettings reads the appearance of the last run, keeping the usual one
// where the file does not read or says what is not allowed.
func (a *App) loadSettings() {
	a.themeMode, a.zoom = "system", 1
	if settingsFile == "" {
		return
	}
	data, err := os.ReadFile(settingsFile)
	if err != nil {
		return
	}
	var s settings
	if err := json.Unmarshal(data, &s); err != nil {
		return
	}
	switch s.Theme {
	case "light", "dark", "system":
		a.themeMode = s.Theme
	}
	if s.Zoom >= minZoom && s.Zoom <= maxZoom {
		a.zoom = s.Zoom
	}
}

// saveSettings writes the appearance now chosen, for the next run.
func (a *App) saveSettings() {
	if settingsFile == "" {
		return
	}
	data, err := json.MarshalIndent(settings{Theme: a.themeMode, Zoom: a.zoom}, "", "  ")
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(settingsFile), 0o755); err != nil {
		return
	}
	tmp := settingsFile + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o644); err != nil {
		return
	}
	_ = os.Rename(tmp, settingsFile)
}

// snapZoom rounds the slider's value to the step it snaps to.
func snapZoom(z float64) float64 {
	z = math.Round(z/zoomStep) * zoomStep
	// 1.5 and the like come out of the arithmetic a hair off.
	return math.Round(z*100) / 100
}
