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

type Meta struct {
	StateDir string
}

func (m Meta) path(branch string) (string, error) {
	dir, ok := worktree.MetaDir(worktree.MetaDirParams{StateDir: m.StateDir, Branch: branch})
	if !ok {
		return "", fmt.Errorf(errNoMetaPath, branch)
	}
	return filepath.Join(dir, metaFileName), nil
}

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

func (m Meta) Update(ctx context.Context, update worktree.MetaUpdate) (domain.Written, error) {
	path, err := m.path(update.Branch)
	if err != nil {
		return domain.Written{}, err
	}
	return statefile.Update(ctx, statefile.UpdateParams{
		Path: path,
		Mutate: func(current []byte) (statefile.Change, error) {
			stored, err := storedMeta(parseParams{Path: path, Current: current})
			if err != nil {
				return nil, err
			}
			if err := update.Change(&stored); err != nil {
				return nil, err
			}
			stored.Schema = 0
			data, err := json.MarshalIndent(stored, "", jsonIndent)
			if err != nil {
				return nil, fmt.Errorf("marshal metadata: %w", err)
			}
			return statefile.Write{Data: data}, nil
		},
	})
}

func (m Meta) LockOrdinals(ctx context.Context, do func() error) (bool, error) {
	return statefile.WithLock(ctx, statefile.LockParams{Path: filepath.Join(m.StateDir, ordinalLock), Do: do})
}

type parseParams struct {
	Path    string
	Current []byte
}

func storedMeta(params parseParams) (worktree.Meta, error) {
	stored, err := decodeMeta(params.Current)
	if params.Current == nil || err != nil {
		return worktree.Meta{}, nil
	}
	if err := statefile.Refusal(statefile.Schema{Path: params.Path, Found: stored.Schema, Supported: metaSchema}); err != nil {
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
