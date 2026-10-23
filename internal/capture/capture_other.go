//go:build !windows && !darwin && !(linux && (amd64 || arm64))

package capture

import "image"

func available() error { return ErrUnsupported }

func captureDisplays([]Display) ([]*image.RGBA, error) { return nil, ErrUnsupported }
