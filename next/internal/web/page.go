package web

import (
	"io/fs"
	"net/http"
	"path"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/domain"
)

// assetsDir holds the front's hashed files, which never change under their name.
const assetsDir = "assets/"

// page serves a file of the front, and its index.html for any other path: the front routes in the browser, so a reload on one of its routes gets the page.
func (s *Server) page(w http.ResponseWriter, r *http.Request) {
	// registered for every method: "GET /" would collide with "/api/" in the mux
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		writeError(w, http.StatusMethodNotAllowed, &domain.Error{Kind: domain.KindRefused, Code: CodeMethodNotAllowed, Message: "the page answers GET only"})
		return
	}

	name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
	info, err := fs.Stat(front, name)
	switch {
	case err == nil && !info.IsDir() && strings.HasPrefix(name, assetsDir):
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	case err == nil && !info.IsDir():
	case strings.HasPrefix(name, assetsDir):
		writeError(w, http.StatusNotFound, &domain.Error{Kind: domain.KindInvalid, Code: CodeNotFound, Message: "no such file: " + name})
		return
	default:
		// a new binary serves new assets on the same origin: the page is asked for again each time
		w.Header().Set("Cache-Control", "no-cache")
		name = "index.html"
	}
	http.ServeFileFS(w, r, front, name)
}
