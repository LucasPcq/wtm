package infra

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/LucasPcq/wtm/internal/domain"
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
