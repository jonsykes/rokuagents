// Package firmware holds the display firmware that is embedded in the helper,
// so that managents flash needs nothing but the helper.
//
// The images are build products, not sources: release builds copy them into
// images/ before compiling (see images/README.md). A build without them has no
// embedded firmware.
package firmware

import (
	"embed"
	"io/fs"
	"strings"
)

//go:embed images
var images embed.FS

// Embedded returns the firmware image for board, written at flash offset 0x0,
// and its version (empty if the build did not record one). ok is false when
// this build embeds no image for the board.
func Embedded(board string) (image []byte, version string, ok bool) {
	return embedded(images, board)
}

// embedded is Embedded for another file system, which has the same layout.
func embedded(fsys fs.FS, board string) (image []byte, version string, ok bool) {
	prefix := "images/managents-firmware-" + board
	image, err := fs.ReadFile(fsys, prefix+".bin")
	if err != nil || len(image) == 0 {
		return nil, "", false
	}
	text, _ := fs.ReadFile(fsys, prefix+".version") // a missing version is not an error
	return image, strings.TrimSpace(string(text)), true
}
