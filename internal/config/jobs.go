package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

// LoadRun reads and parses run.toml from the state directory.
// Returns an empty config (no error) if the file does not exist. Unknown
// keys (typos like `[[profiles]]` instead of `[[profile]]`) surface as
// errors rather than being silently ignored.
//
// The port declarations are checked here rather than only on write: an
// unworkable layout costs an EADDRINUSE that names nothing, so reading the file
// is the last moment it can be explained. The structural checks stay on write —
// making a duplicate job name fatal here would lock the user out of the very
// commands that repair the file.
func LoadRun(stateDir string) (domain.RunConfig, error) {
	path := filepath.Join(stateDir, domain.RunFileName)

	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return domain.RunConfig{}, nil
	}

	var cfg domain.RunConfig
	if err := decodeStrict(path, &cfg); err != nil {
		return domain.RunConfig{}, err
	}

	// Enforced on read like the port layout, and for the same reason: an
	// addressing nobody recognizes would read as the default and write the wrong
	// thing into a .env, with nothing pointing back at run.toml.
	errs := append(rules.ValidateRunPorts(cfg), rules.ValidateAddressing(cfg)...)
	if errs = append(errs, rules.ValidateTenants(cfg)...); len(errs) > 0 {
		return domain.RunConfig{}, fmt.Errorf("invalid run config: %s", strings.Join(errs, "; "))
	}

	return cfg, nil
}
