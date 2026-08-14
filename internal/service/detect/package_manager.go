package detect

import (
	"path/filepath"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra"
)

type lockfileCheck struct {
	lockfile string
	pm       domain.PackageManager
}

// lockfileChecks is the lockfile → package-manager priority order, shared by the
// root probe and per-directory sub-project discovery.
var lockfileChecks = []lockfileCheck{
	{domain.LockfilePnpm, domain.PkgManagerPnpm},
	{domain.LockfileNpm, domain.PkgManagerNpm},
	{domain.LockfileYarn, domain.PkgManagerYarn},
	{domain.LockfileGo, domain.PkgManagerGo},
	{domain.LockfilePip, domain.PkgManagerPip},
}

// PackageManager detects the package manager from lockfiles in the given directory.
// Checks in priority order: pnpm, npm, yarn, go, pip. Directory-scoped: it answers
// "how does this exact directory install", so it serves both the project root and
// each independent sub-project.
func PackageManager(projectDir string) domain.PackageManager {
	for _, check := range lockfileChecks {
		if infra.FileExists(filepath.Join(projectDir, check.lockfile)) {
			return check.pm
		}
	}

	return domain.PkgManagerNone
}
