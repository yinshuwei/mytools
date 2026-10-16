package tools

// The picture formats the tools read. Both ui.DecodeBitmap and
// image.DecodeConfig go by what is registered, so they are registered here,
// once, for every tool that reads a picture. PNG is not among them: it is
// imported where it is used, as a codec of its own.
import (
	_ "image/gif"
	_ "image/jpeg"

	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/webp"
)
