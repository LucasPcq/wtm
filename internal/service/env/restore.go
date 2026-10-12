package env

import (
	"context"
	"path/filepath"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/store"
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
	files, err := planRestore(params)
	if err != nil {
		return nil, err
	}
	return restoredEntries(files), nil
}

// ApplyOwnedRestore puts the owned keys back to the source's values — what a
// verbatim worktree's .env holds — and returns what it changed.
func ApplyOwnedRestore(ctx context.Context, params OwnedRestoreParams) ([]domain.EnvRestoredEntry, error) {
	files, err := planRestore(params)
	if err != nil {
		return nil, err
	}
	for _, file := range files {
		if len(file.Entries) == 0 {
			continue
		}
		if _, err := (store.EnvDisk{}).Edit(ctx, store.EnvEdit{Path: file.Path, Edit: file.Restore}); err != nil {
			return nil, err
		}
	}
	return restoredEntries(files), nil
}

type restoredFile struct {
	Path    string
	Entries []domain.EnvRestoredEntry
	// Restore puts the owned keys back in the lines read under the file's lock.
	Restore func([]domain.EnvLine) []domain.EnvLine
}

func restoredEntries(files []restoredFile) []domain.EnvRestoredEntry {
	var restored []domain.EnvRestoredEntry
	for _, file := range files {
		restored = append(restored, file.Entries...)
	}
	return restored
}

func planRestore(params OwnedRestoreParams) ([]restoredFile, error) {
	paths := envPaths{
		MainPath:           params.MainPath,
		WorktreePath:       params.WorktreePath,
		ParentWorktreePath: params.ParentWorktreePath,
		Strategy:           params.Strategy,
	}

	var files []restoredFile
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

		restore := rules.RestoreOwnedEnvParams{File: file.Target, Child: child, Source: source, Keys: keys, PortBases: portBasesOf(params.Keys.PortBases, file.Target)}
		_, entries := rules.RestoreOwnedEnv(restore)
		files = append(files, restoredFile{Path: path, Entries: entries, Restore: func(current []domain.EnvLine) []domain.EnvLine {
			restore.Child = current
			lines, _ := rules.RestoreOwnedEnv(restore)
			return lines
		}})
	}
	return files, nil
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
