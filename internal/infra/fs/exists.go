package fs

import (
	"os"
	"path/filepath"
)

// FileExists reports whether a file exists at the given path.
func FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// ResolvePath follows symlinks so two spellings of the same directory compare
// equal. On macOS a working directory reached through /var and a git worktree
// path reported as /private/var are the same place, and a raw string compare
// would silently say otherwise. Returns path unchanged when it cannot be
// resolved: an unresolvable path is not worth failing a comparison over.
func ResolvePath(path string) string {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return path
	}
	return resolved
}
