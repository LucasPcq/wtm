package config

import (
	"bytes"
	"context"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"

	"github.com/LucasPcq/wtm/internal/domain"
	domainstatefile "github.com/LucasPcq/wtm/internal/domain/statefile"
	"github.com/LucasPcq/wtm/internal/infra/fs"
	"github.com/LucasPcq/wtm/internal/rules/statefile"
)

// pendingRemovalsFile is the queue of namespaces a clean could not give back. A
// worktree's own state directory is exactly what clean removes, so the debt it
// leaves has to live beside the repository rather than inside the worktree that
// incurred it.
type pendingRemovalsFile struct {
	Schema  *int                  `toml:"schema,omitempty"`
	Pending []domain.NamespaceRef `toml:"pending"`
}

func pendingRemovalsPath(stateDir string) string {
	return filepath.Join(stateDir, domain.PendingRemovalsFileName)
}

// LoadPendingRemovals reads the queue. An unreadable or absent file is an empty
// one: a debt wtm cannot read is one it cannot settle either, and failing
// `prune` over it would help nobody.
func LoadPendingRemovals(stateDir string) []domain.NamespaceRef {
	data, err := os.ReadFile(pendingRemovalsPath(stateDir))
	if err != nil {
		return nil
	}
	return decodePendingRemovals(data)
}

func decodePendingRemovals(data []byte) []domain.NamespaceRef {
	var file pendingRemovalsFile
	if _, err := toml.Decode(string(data), &file); err != nil {
		return nil
	}
	return file.Pending
}

type UpdatePendingRemovalsParams struct {
	StateDir string
	Update   func([]domain.NamespaceRef) []domain.NamespaceRef
}

// UpdatePendingRemovals changes the queue as it is on disk at the moment of the
// write, and removes the file once it is empty so nothing is left claiming a
// debt that is settled.
func UpdatePendingRemovals(ctx context.Context, params UpdatePendingRemovalsParams) error {
	path := pendingRemovalsPath(params.StateDir)
	_, err := fs.UpdateFile(ctx, fs.UpdateFileParams{
		Path: path,
		Mutate: func(current []byte) (fs.Change, error) {
			if err := refuseNewer(refuseNewerParams{Path: path, Current: current, Supported: domainstatefile.PendingRemovalsSchema}); err != nil {
				return nil, err
			}
			next := params.Update(decodePendingRemovals(current))
			if len(next) == 0 {
				return fs.Remove{}, nil
			}
			var buf bytes.Buffer
			if err := toml.NewEncoder(&buf).Encode(pendingRemovalsFile{
				Schema:  nonZero(statefile.Written(domainstatefile.PendingRemovalsSchema)),
				Pending: next,
			}); err != nil {
				return nil, err
			}
			return fs.Write{Data: buf.Bytes()}, nil
		},
	})
	return err
}
