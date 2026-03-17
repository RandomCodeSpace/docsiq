package ui

import (
	"embed"
	"io/fs"
)

//go:embed dist
var dist embed.FS

var Assets, _ = fs.Sub(dist, "dist")
