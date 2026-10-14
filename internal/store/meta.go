// Package store is the one writer of each state file: a worktree's meta.json, the changes to config.toml and the .env files wtm manages.
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

	"github.com/LucasPcq/wtm/internal/domain"
	domainstatefile "github.com/LucasPcq/wtm/internal/domain/statefile"
	"github.com/LucasPcq/wtm/internal/infra/fs"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/rules/statefile"
)

// Meta is the record wtm keeps of each worktree, <common>/wtm/worktrees/<branch>/meta.json.
type Meta interface {
	// Load is the record on disk; an absent one is an error wrapping os.ErrNotExist.
	Load(branch string) (domain.WorktreeMetadata, error)
	// Records are every record that reads, by branch; one that does not read is left out.
	Records() ([]MetaRecord, error)
	Update(ctx context.Context, update MetaUpdate) (Written, error)
	// Purge drops the record and its directory: nothing is left to hold an ordinal.
	Purge(branch string) error
}

type MetaRecord struct {
	Branch string
	Meta   domain.WorktreeMetadata
}

// Missing is what Update does with a record it finds absent.
type Missing int

// The zero value refuses: a writer that creates a record says so.
const (
	// MissingRefuse returns an error wrapping os.ErrNotExist.
	MissingRefuse Missing = iota
	// MissingCreate starts from the zero record, also when the stored one does not read.
	MissingCreate
	// MissingSkip writes nothing: the main checkout never gets a record.
	MissingSkip
)

type MetaUpdate struct {
	Branch  string
	Missing Missing
	// Change edits the record read under the lock; one left unchanged is not rewritten.
	Change func(*domain.WorktreeMetadata) error
}

// MetaDisk keeps the records under StateDir, <common>/wtm.
type MetaDisk struct {
	StateDir string
}

var _ Meta = MetaDisk{}

func (m MetaDisk) path(branch string) string {
	return filepath.Join(rules.WorktreeMetaDir(m.StateDir, branch), domain.MetaFileName)
}

// Load reads without the lock: a write renames a whole file over the record.
func (m MetaDisk) Load(branch string) (domain.WorktreeMetadata, error) {
	data, err := os.ReadFile(m.path(branch))
	if err != nil {
		return domain.WorktreeMetadata{}, err
	}
	return decodeMeta(data)
}

// Records reads every record on disk rather than the live worktrees: a record
// is what clean gives back by, whether or not git still lists its worktree.
func (m MetaDisk) Records() ([]MetaRecord, error) {
	root := filepath.Join(m.StateDir, domain.WorktreesSubdir)
	entries, err := os.ReadDir(root)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var records []MetaRecord
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(root, entry.Name(), domain.MetaFileName))
		if err != nil {
			continue
		}
		meta, err := decodeMeta(data)
		if err != nil {
			continue
		}
		branch, err := url.PathUnescape(entry.Name())
		if err != nil {
			branch = entry.Name()
		}
		records = append(records, MetaRecord{Branch: branch, Meta: meta})
	}
	sort.Slice(records, func(i, j int) bool { return records[i].Branch < records[j].Branch })
	return records, nil
}

// Update is the one write of a record: a read-modify-write under its lock, so
// two commands changing different fields of the same worktree both land.
func (m MetaDisk) Update(ctx context.Context, update MetaUpdate) (Written, error) {
	path := m.path(update.Branch)
	changed := false
	lock, err := fs.UpdateFile(ctx, fs.UpdateFileParams{
		Path: path,
		Mutate: func(current []byte) (fs.Change, error) {
			if current == nil && update.Missing == MissingSkip {
				return fs.Keep{}, nil
			}
			stored, err := storedMeta(storedMetaParams{Path: path, Current: current, Missing: update.Missing})
			if err != nil {
				return nil, err
			}
			if err := update.Change(&stored); err != nil {
				return nil, err
			}
			stored.Schema = statefile.Written(domainstatefile.MetaSchema)
			data, err := json.MarshalIndent(stored, "", "  ")
			if err != nil {
				return nil, fmt.Errorf("marshal metadata: %w", err)
			}
			if bytes.Equal(current, data) {
				return fs.Keep{}, nil
			}
			changed = true
			return fs.Write{Data: data}, nil
		},
	})
	if err != nil {
		return Written{}, err
	}
	return Written{Changed: changed, Unlocked: !lock.Held}, nil
}

// Purge is best effort for its caller, a removal that already happened, but
// says what it could not remove.
func (m MetaDisk) Purge(branch string) error {
	dir := rules.PurgeableMetaDir(rules.PurgeableMetaDirParams{StateDir: m.StateDir, Branch: branch})
	if dir == "" {
		return nil
	}
	return fs.RemoveDir(dir)
}

type storedMetaParams struct {
	Path    string
	Current []byte
	Missing Missing
}

func storedMeta(params storedMetaParams) (domain.WorktreeMetadata, error) {
	if params.Current == nil {
		if params.Missing == MissingRefuse {
			return domain.WorktreeMetadata{}, fmt.Errorf("read %s: %w", params.Path, os.ErrNotExist)
		}
		return domain.WorktreeMetadata{}, nil
	}
	stored, err := decodeMeta(params.Current)
	if err != nil {
		if params.Missing == MissingCreate {
			return domain.WorktreeMetadata{}, nil
		}
		return domain.WorktreeMetadata{}, fmt.Errorf("read %s: %w", params.Path, err)
	}
	if err := statefile.Refusal(statefile.Schema{Path: params.Path, Found: stored.Schema, Supported: domainstatefile.MetaSchema}); err != nil {
		return domain.WorktreeMetadata{}, err
	}
	return stored, nil
}

func decodeMeta(data []byte) (domain.WorktreeMetadata, error) {
	var meta domain.WorktreeMetadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return domain.WorktreeMetadata{}, err
	}
	return meta, nil
}
