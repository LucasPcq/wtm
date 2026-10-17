// Package store reads and writes a worktree's meta.json and the .env files wtm provisions.
package store

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/LucasPcq/wtm/next/internal/adapter/statefile"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

const (
	metaFileName  = "meta.json"
	ordinalLock   = "ordinal.lock"
	metaSchema    = 1
	jsonIndent    = "  "
	errNoMetaPath = "no meta.json path for branch %q"
)

// Meta reads and writes the meta.json record of each worktree, under the
// project's state directory StateDir.
type Meta struct {
	StateDir string
}

// path returns where the record of branch lives, or an error for a branch name
// that maps onto no directory.
func (m Meta) path(branch string) (string, error) {
	dir, ok := worktree.MetaDir(worktree.MetaDirParams{StateDir: m.StateDir, Branch: branch})
	if !ok {
		return "", fmt.Errorf(errNoMetaPath, branch)
	}

	return filepath.Join(dir, metaFileName), nil
}

// Load returns the record of branch. A missing record is an error wrapping
// os.ErrNotExist; a record that does not decode is the JSON error.
func (m Meta) Load(branch string) (worktree.Meta, error) {
	path, err := m.path(branch)
	if err != nil {
		return worktree.Meta{}, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return worktree.Meta{}, err
	}

	return decodeMeta(data)
}

// Update applies update.Change to the record of update.Branch under its lock,
// starting from an empty record when there is none or it does not decode, and
// returns what it wrote. A record written by a newer wtm is refused with a
// *domain.SchemaNewer, and an error of Change leaves the record as it was.
func (m Meta) Update(ctx context.Context, update worktree.MetaUpdate) (domain.Written, error) {
	path, err := m.path(update.Branch)
	if err != nil {
		return domain.Written{}, err
	}

	return statefile.Update(ctx, statefile.UpdateParams{
		Path: path,
		Mutate: func(current []byte) (statefile.Change, error) {
			stored, err := storedMeta(path, current)
			if err != nil {
				return nil, err
			}
			if err := update.Change(&stored); err != nil {
				return nil, err
			}

			// Schema 1 is written as no schema at all, the way wtm writes it.
			stored.Schema = 0
			data, err := json.MarshalIndent(stored, "", jsonIndent)
			if err != nil {
				return nil, fmt.Errorf("marshal metadata: %w", err)
			}
			return statefile.Write{Data: data}, nil
		},
	})
}

// LockOrdinals runs do holding the project's ordinal lock, so two processes
// never hand the same ordinal to two worktrees, and reports whether the lock
// was held (do runs unlocked where it cannot be taken).
func (m Meta) LockOrdinals(ctx context.Context, do func() error) (bool, error) {
	return statefile.WithLock(ctx, filepath.Join(m.StateDir, ordinalLock), do)
}

// storedMeta returns the record current holds, an empty one when there is
// none or it does not decode, so a broken record is rewritten rather than
// blocking every write; a record of a newer schema is refused.
func storedMeta(path string, current []byte) (worktree.Meta, error) {
	stored, err := decodeMeta(current)
	if current == nil || err != nil {
		return worktree.Meta{}, nil
	}

	if err := statefile.Refusal(statefile.Schema{Path: path, Found: stored.Schema, Supported: metaSchema}); err != nil {
		return worktree.Meta{}, err
	}

	return stored, nil
}

func decodeMeta(data []byte) (worktree.Meta, error) {
	var meta worktree.Meta
	if err := json.Unmarshal(data, &meta); err != nil {
		return worktree.Meta{}, err
	}
	return meta, nil
}
