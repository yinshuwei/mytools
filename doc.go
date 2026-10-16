// Package mytools is the toolbox as a whole. It holds the files the window
// itself shows, embedded so that the binary carries them: the settings panel
// works wherever the program is run from, not only from the source tree.
package mytools

import _ "embed"

// Readme is README.md, as it is on disk: what the settings panel's Help
// shows.
//
//go:embed README.md
var Readme string
