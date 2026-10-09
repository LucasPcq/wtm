package env

import (
	"path/filepath"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

// OwnedRestoreParams locates a worktree, the source its .env files were copied
// from, and the keys wtm owns in them.
type OwnedRestoreParams struct {
	MainPath           string
	WorktreePath       string
	ParentWorktreePath string
	Strategy           domain.EnvStrategy
	Files              []domain.EnvFile
	Keys               domain.EnvOwnedKeys
}

// PlanOwnedRestore is what putting the owned keys back to the source's values
// would change, writing nothing.
func PlanOwnedRestore(params OwnedRestoreParams) ([]domain.EnvRestoredEntry, error) {
	return restoreOwned(params, false)
}

// ApplyOwnedRestore puts the owned keys back to the source's values — what a
// verbatim worktree's .env holds — and returns what it changed.
func ApplyOwnedRestore(params OwnedRestoreParams) ([]domain.EnvRestoredEntry, error) {
	return restoreOwned(params, true)
}

func restoreOwned(params OwnedRestoreParams, write bool) ([]domain.EnvRestoredEntry, error) {
	paths := envPaths{
		MainPath:           params.MainPath,
		WorktreePath:       params.WorktreePath,
		ParentWorktreePath: params.ParentWorktreePath,
		Strategy:           params.Strategy,
	}

	var restored []domain.EnvRestoredEntry
	for _, file := range params.Files {
		keys := keysOf(params.Keys.Refs, file.Target)
		if len(keys) == 0 {
			continue
		}
		path := filepath.Join(params.WorktreePath, file.Target)
		child, err := readEnvFile(path)
		if err != nil {
			return nil, err
		}
		if child == nil {
			continue
		}
		source, err := provisioningSource(paths, file)
		if err != nil {
			return nil, err
		}

		lines, entries := rules.RestoreOwnedEnv(rules.RestoreOwnedEnvParams{File: file.Target, Child: child, Source: source, Keys: keys, PortBases: portBasesOf(params.Keys.PortBases, file.Target)})
		restored = append(restored, entries...)
		if !write || len(entries) == 0 {
			continue
		}
		if err := writeEnvFile(path, rules.RenderEnv(lines)); err != nil {
			return nil, err
		}
	}
	return restored, nil
}

// provisioningSource is the document a verbatim worktree's file is a copy of:
// the one `wtm create` would copy under the same strategy.
func provisioningSource(paths envPaths, file domain.EnvFile) ([]domain.EnvLine, error) {
	parent, main, _, _, err := valueSources(paths, file)
	switch {
	case err != nil:
		return nil, err
	case parent != nil:
		return parent, nil
	case main != nil:
		return main, nil
	default:
		return templateLines(paths.MainPath, file)
	}
}

func portBasesOf(bases map[domain.EnvKeyRef][]int, file string) map[string][]int {
	mine := map[string][]int{}
	for ref, ports := range bases {
		if ref.File == file {
			mine[ref.Key] = ports
		}
	}
	return mine
}

func keysOf(refs []domain.EnvKeyRef, file string) []string {
	var keys []string
	for _, ref := range refs {
		if ref.File == file {
			keys = append(keys, ref.Key)
		}
	}
	return keys
}
