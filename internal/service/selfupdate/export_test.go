package selfupdate

import "context"

func init() {
	allowInsecureDownload = true
}

// GoBinDirForTest exposes the go-install destination resolution, whose symlink
// handling is what keeps a GOBIN under /tmp (a symlink on macOS) comparable to
// the resolved executable path.
func GoBinDirForTest() string { return goBinDir(context.Background()) }
