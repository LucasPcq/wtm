package fs

import (
	"errors"
	"os"
	"path/filepath"
)

// Exists is false only for a path that is not there: a path that cannot be
// stat'ed for another reason is an error, never taken for absent.
func Exists(path string) (bool, error) {
	_, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// FileExists takes any error for absence, for the commands still on flow/.
func FileExists(path string) bool {
	exists, err := Exists(path)
	return exists && err == nil
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
