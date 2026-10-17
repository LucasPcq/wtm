package detect

import (
	"path/filepath"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra/fs"
)

// PackageManager detects the package manager from lockfiles in the project directory.
// Checks in priority order: pnpm, npm, yarn, go, pip.
func PackageManager(projectDir string) domain.PackageManager {
	checks := []struct {
		lockfile string
		pm       domain.PackageManager
	}{
		{domain.LockfilePnpm, domain.PkgManagerPnpm},
		{domain.LockfileNpm, domain.PkgManagerNpm},
		{domain.LockfileYarn, domain.PkgManagerYarn},
		{domain.LockfileGo, domain.PkgManagerGo},
		{domain.LockfilePip, domain.PkgManagerPip},
	}

	for _, check := range checks {
		if fs.FileExists(filepath.Join(projectDir, check.lockfile)) {
			return check.pm
		}
	}

	return domain.PkgManagerNone
}
