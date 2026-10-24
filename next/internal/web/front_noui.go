//go:build noui

package web

import (
	"embed"
	"io/fs"
)

// noui is the one page of a binary built without the front (-tags noui).
//
//go:embed noui
var noui embed.FS

// front is the page and its assets, served from the root of the server.
var front, _ = fs.Sub(noui, "noui") // fs.Sub fails on an invalid name only
