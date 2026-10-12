// Package store is the one writer of each state file: a worktree's meta.json, the remembered answers in config.toml and the .env files wtm manages.
package store

import (
	"context"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

// Meta is the record wtm keeps of each worktree, <common>/wtm/worktrees/<branch>/meta.json.
type Meta interface {
	// Load is the record on disk; an absent one is an error wrapping os.ErrNotExist.
	Load(branch string) (domain.WorktreeMetadata, error)
	// Records are every record that reads, by branch; one that does not read is left out.
	Records() ([]MetaRecord, error)
	Update(ctx context.Context, update MetaUpdate) error
	// Purge drops the record and its directory: nothing is left to hold an ordinal.
	Purge(branch string) error
}

// ProjectConfig is config.toml, changed as it is on disk under its lock.
type ProjectConfig interface {
	Remember(ctx context.Context, change rules.RememberedChange) error
}

// EnvFiles are the .env files of a worktree, written under a lock kept outside the user's tree.
type EnvFiles interface {
	Copy(ctx context.Context, request EnvCopy) ([]EnvWarning, error)
	Edit(ctx context.Context, edit EnvEdit) (changed bool, err error)
}

type MetaRecord struct {
	Branch string
	Meta   domain.WorktreeMetadata
}

// Missing is what Update does with a record it finds absent.
type Missing int

const (
	// MissingCreate starts from the zero record, also when the stored one does not read.
	MissingCreate Missing = iota
	// MissingSkip writes nothing: the main checkout never gets a record.
	MissingSkip
	// MissingRefuse returns an error wrapping os.ErrNotExist.
	MissingRefuse
)

type MetaUpdate struct {
	Branch  string
	Missing Missing
	// Change edits the record read under the lock; one left unchanged is not rewritten.
	Change func(*domain.WorktreeMetadata) error
}

type EnvCopy struct {
	Strategy           domain.EnvStrategy
	Files              []domain.EnvFile
	TargetDir          string
	MainWorktreePath   string
	ParentWorktreePath string
}

type EnvEdit struct {
	Path string
	// Create writes a file that does not exist; otherwise an absent file stays absent.
	Create bool
	// Edit receives the lines read under the lock, nil for an absent file.
	Edit func([]domain.EnvLine) []domain.EnvLine
}

// EnvWarning is a file Copy could not provision as its strategy asked.
//
// go-sumtype:decl EnvWarning
type EnvWarning interface{ envWarning() }

// NoTemplate is a file the example strategy skipped: no template on disk.
type NoTemplate struct{ Target string }

// SourceMissing is a file skipped because Dir does not hold it.
type SourceMissing struct {
	Target string
	Dir    string
}

// ParentFallback is a file the parent worktree lacks, copied from the main one.
type ParentFallback struct{ Target string }

func (NoTemplate) envWarning()     {}
func (SourceMissing) envWarning()  {}
func (ParentFallback) envWarning() {}
