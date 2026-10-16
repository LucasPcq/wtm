package hook

import (
	"net/url"
	"path/filepath"
)

const (
	logDir   = "hooks"
	logPhase = "on_create"
	logExt   = ".log"
)

type LogPathParams struct {
	StateDir string
	Branch   string
}

func LogPath(params LogPathParams) string {
	if params.StateDir == "" {
		return ""
	}
	return filepath.Join(params.StateDir, logDir, logPhase+"-"+url.PathEscape(params.Branch)+logExt)
}
