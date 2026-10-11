package fs

import (
	"os"
	"path/filepath"
)

// OpenLog creates the log at path for a writer that streams into it, replacing
// the one a previous run left. A log is not a state file: nothing reads it back
// to decide, so it is written as the output comes rather than through
// UpdateFile.
func OpenLog(path string) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	return os.Create(path)
}
