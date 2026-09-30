package shared

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra"
)

func StateDir(dir string) (string, error) {
	if override := os.Getenv(domain.EnvStateDir); override != "" {
		return override, nil
	}
	commonDir, err := infra.GitCommonDir(infra.GitCommonDirParams{Dir: dir})
	if err != nil {
		return "", fmt.Errorf("resolve state dir: %w", err)
	}
	return filepath.Join(commonDir, domain.StateDirName), nil
}
