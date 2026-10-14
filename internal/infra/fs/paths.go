// Package fs is the only writer of the files wtm keeps.
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

// GlobalDir is resolved, never spelled: the user-config directory is ~/.config
// on Linux and ~/Library/Application Support on macOS.
func GlobalDir() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("locate user config dir: %w", err)
	}
	return filepath.Join(configDir, domain.GlobalConfigDir), nil
}

func ScratchLockPath(path string) string {
	sum := sha256.Sum256([]byte(resolvedFile(path)))
	return filepath.Join(os.TempDir(), scratchLockDir, hex.EncodeToString(sum[:16])+lockSuffix)
}

// resolvedFile resolves the directory when the file does not exist yet: its
// first write and the ones after must take the same lock.
func resolvedFile(path string) string {
	absolute, err := filepath.Abs(path)
	if err != nil {
		absolute = path
	}
	if resolved, err := filepath.EvalSymlinks(absolute); err == nil {
		return resolved
	}
	return filepath.Join(ResolvePath(filepath.Dir(absolute)), filepath.Base(absolute))
}

// ResolvePath follows symlinks: on macOS /var and /private/var are the same
// place. A path it cannot resolve is returned as it is.
func ResolvePath(path string) string {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return path
	}
	return resolved
}
