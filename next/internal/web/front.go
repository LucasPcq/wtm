//go:build !noui

package web

import (
	"embed"
	"io/fs"
)

// dist is the front's build, which `make ui` writes from next/web.
//
//go:embed all:dist
var dist embed.FS

// front is the page and its assets, served from the root of the server.
var front, _ = fs.Sub(dist, "dist") // fs.Sub fails on an invalid name only
