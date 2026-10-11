// Package fs is the only writer of the files wtm keeps: atomic, locked updates and the paths they live at.
package fs

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	"github.com/LucasPcq/wtm/internal/domain"
)

const (
	lockSuffix     = ".lock"
	scratchLockDir = "wtm-locks"
)

// GlobalDir is wtm's own directory under the OS user-config directory. It is
// resolved, never spelled: os.UserConfigDir is ~/.config on Linux and
// ~/Library/Application Support on macOS, so any literal path is wrong on one
// of the two.
func GlobalDir() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("locate user config dir: %w", err)
	}
	return filepath.Join(configDir, domain.GlobalConfigDir), nil
}

// ScratchLockPath is the lock of a file wtm writes in the user's tree: in the
// temporary directory, named after the file's absolute path, so every process
// updating that file meets on the same lock.
func ScratchLockPath(path string) string {
	absolute, err := filepath.Abs(path)
	if err != nil {
		absolute = path
	}
	sum := sha256.Sum256([]byte(absolute))
	return filepath.Join(os.TempDir(), scratchLockDir, hex.EncodeToString(sum[:16])+lockSuffix)
}
