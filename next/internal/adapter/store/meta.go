// Package store reads and writes a worktree's meta.json and the .env files wtm provisions.
package store

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"

	"github.com/LucasPcq/wtm/next/internal/adapter/statefile"
	"github.com/LucasPcq/wtm/next/internal/wtm"
	"github.com/LucasPcq/wtm/next/internal/wtm/worktree"
)

const (
	metaFileName  = "meta.json"
	metaDirName   = "worktrees"
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

func (m Meta) Records() ([]worktree.MetaRecord, error) {
	root := filepath.Join(m.StateDir, metaDirName)
	entries, err := os.ReadDir(root)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var records []worktree.MetaRecord
	for _, entry := range entries {
		record, ok := readRecord(filepath.Join(root, entry.Name()), entry)
		if ok {
			records = append(records, record)
		}
	}
	sort.Slice(records, func(i, j int) bool { return records[i].Branch < records[j].Branch })
	return records, nil
}

func readRecord(dir string, entry os.DirEntry) (worktree.MetaRecord, bool) {
	if !entry.IsDir() {
		return worktree.MetaRecord{}, false
	}
	data, err := os.ReadFile(filepath.Join(dir, metaFileName))
	if err != nil {
		return worktree.MetaRecord{}, false
	}
	meta, err := decodeMeta(data)
	if err != nil {
		return worktree.MetaRecord{}, false
	}
	branch, err := url.PathUnescape(entry.Name())
	if err != nil {
		branch = entry.Name()
	}
	return worktree.MetaRecord{Branch: branch, Meta: meta}, true
}

func (m Meta) Update(ctx context.Context, update worktree.MetaUpdate) (wtm.Written, error) {
	path, err := m.path(update.Branch)
	if err != nil {
		return wtm.Written{}, err
	}
	changed := false
	lock, err := statefile.Update(ctx, statefile.UpdateParams{
		Path: path,
		Mutate: func(current []byte) (statefile.Change, error) {
			if current == nil && update.Missing == worktree.MissingSkip {
				return statefile.Keep{}, nil
			}
			stored, err := storedMeta(storedMetaParams{Path: path, Current: current, Missing: update.Missing})
			if err != nil {
				return nil, err
			}
			if err := update.Change(&stored); err != nil {
				return nil, err
			}
			stored.Schema = statefile.Stamp(metaSchema)
			data, err := json.MarshalIndent(stored, "", jsonIndent)
			if err != nil {
				return nil, fmt.Errorf("marshal metadata: %w", err)
			}
			if bytes.Equal(current, data) {
				return statefile.Keep{}, nil
			}
			changed = true
			return statefile.Write{Data: data}, nil
		},
	})
	if err != nil {
		return wtm.Written{}, err
	}
	return wtm.Written{Changed: changed, Unlocked: !lock.Held}, nil
}

func (m Meta) Purge(branch string) error {
	dir, ok := worktree.MetaDir(worktree.MetaDirParams{StateDir: m.StateDir, Branch: branch})
	if !ok {
		return nil
	}
	return statefile.RemoveDir(dir)
}

type storedMetaParams struct {
	Path    string
	Current []byte
	Missing worktree.Missing
}

func storedMeta(params storedMetaParams) (worktree.Meta, error) {
	if params.Current == nil && params.Missing == worktree.MissingRefuse {
		return worktree.Meta{}, fmt.Errorf("read %s: %w", params.Path, os.ErrNotExist)
	}
	if params.Current == nil {
		return worktree.Meta{}, nil
	}
	stored, err := decodeMeta(params.Current)
	if err != nil && params.Missing == worktree.MissingCreate {
		return worktree.Meta{}, nil
	}
	if err != nil {
		return worktree.Meta{}, fmt.Errorf("read %s: %w", params.Path, err)
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
