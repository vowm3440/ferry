package ui

import (
	"embed"
	"io/fs"
)

// Build scripts copy frontend/dist here before compiling the GUI.
//
//go:embed all:dist
var assets embed.FS

func Assets() fs.FS {
	content, err := fs.Sub(assets, "dist")
	if err != nil {
		panic("embedded frontend is missing")
	}
	return content
}
