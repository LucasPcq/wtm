package config

import (
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"

	"github.com/LucasPcq/wtm/internal/domain"
)

// pendingDetachFile is the queue of tenants a clean could not give back. A
// worktree's own state directory is exactly what clean removes, so the debt it
// leaves has to live beside the repository rather than inside the worktree that
// incurred it.
type pendingDetachFile struct {
	Pending []domain.TenantRef `toml:"pending"`
}

func pendingDetachPath(stateDir string) string {
	return filepath.Join(stateDir, domain.PendingDetachFileName)
}

// LoadPendingDetach reads the queue. An unreadable or absent file is an empty
// one: a debt wtm cannot read is one it cannot settle either, and failing
// `prune` over it would help nobody.
func LoadPendingDetach(stateDir string) []domain.TenantRef {
	var file pendingDetachFile
	if _, err := toml.DecodeFile(pendingDetachPath(stateDir), &file); err != nil {
		return nil
	}
	return file.Pending
}

type WritePendingDetachParams struct {
	StateDir string
	Refs     []domain.TenantRef
}

// WritePendingDetach replaces the queue, removing the file once it is empty so
// nothing is left claiming a debt that is settled.
func WritePendingDetach(params WritePendingDetachParams) error {
	path := pendingDetachPath(params.StateDir)
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
	return toml.NewEncoder(file).Encode(pendingDetachFile{Pending: params.Refs})
}
