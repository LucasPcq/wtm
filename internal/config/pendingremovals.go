package config

import (
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"

	"github.com/LucasPcq/wtm/internal/domain"
)

// pendingRemovalsFile is the queue of namespaces a clean could not give back. A
// worktree's own state directory is exactly what clean removes, so the debt it
// leaves has to live beside the repository rather than inside the worktree that
// incurred it.
type pendingRemovalsFile struct {
	Pending []domain.NamespaceRef `toml:"pending"`
}

func pendingRemovalsPath(stateDir string) string {
	return filepath.Join(stateDir, domain.PendingRemovalsFileName)
}

// LoadPendingRemovals reads the queue. An unreadable or absent file is an empty
// one: a debt wtm cannot read is one it cannot settle either, and failing
// `prune` over it would help nobody.
func LoadPendingRemovals(stateDir string) []domain.NamespaceRef {
	var file pendingRemovalsFile
	if _, err := toml.DecodeFile(pendingRemovalsPath(stateDir), &file); err != nil {
		return nil
	}
	return file.Pending
}

type WritePendingRemovalsParams struct {
	StateDir string
	Refs     []domain.NamespaceRef
}

// WritePendingRemovals replaces the queue, removing the file once it is empty so
// nothing is left claiming a debt that is settled.
func WritePendingRemovals(params WritePendingRemovalsParams) error {
	path := pendingRemovalsPath(params.StateDir)
	if len(params.Refs) == 0 {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}

	if err := os.MkdirAll(params.StateDir, 0o755); err != nil {
		return err
	}
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	return toml.NewEncoder(file).Encode(pendingRemovalsFile{Pending: params.Refs})
}
