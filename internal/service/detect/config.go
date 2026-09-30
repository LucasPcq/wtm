package detect

import (
	"path/filepath"

	"github.com/LucasPcq/wtm/internal/config"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra"
)

// GlobalConfigExists checks whether the machine-level config file is there.
func GlobalConfigExists() bool {
	path := config.GlobalPath()
	return path != "" && infra.FileExists(path)
}

// ProjectConfigExists checks whether config.toml exists in the given state dir.
func ProjectConfigExists(stateDir string) bool {
	return infra.FileExists(filepath.Join(stateDir, domain.ConfigFileName))
}
