package fs

import (
	"os"
	"path/filepath"
)

// OpenLog is not UpdateFile: a log is streamed as the output comes, and
// nothing reads it back to decide.
func OpenLog(path string) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	return os.Create(path)
}
