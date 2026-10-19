// Package mytools is the toolbox as a whole. It holds the files the window
// itself shows, embedded so that the binary carries them: the settings panel
// works wherever the program is run from, not only from the source tree.
package mytools

import "embed"

// Readme is README.md, as it is on disk: what the settings panel's Help
// shows.
//
//go:embed README.md
var Readme string

// Shots holds the screenshots the readme shows. They are embedded for the
// same reason the readme is, and the Help page inlines them into the page it
// renders: the pictures are there wherever the program is run from, not only
// from the source tree, and the web view needs no access to the disk to draw
// them.
//
//go:embed docs/*.png
var Shots embed.FS
