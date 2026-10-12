package store

import (
	"context"
	"fmt"
	"os"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra/fs"
	"github.com/LucasPcq/wtm/internal/rules"
)

// EnvFiles are the .env files of a worktree, written under a lock kept outside the user's tree.
type EnvFiles interface {
	Copy(ctx context.Context, request EnvCopy) ([]EnvWarning, error)
	Edit(ctx context.Context, edit EnvEdit) (changed bool, err error)
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

// EnvDisk writes the .env files in place, each locked by fs.ScratchLockPath:
// a lock file beside it would show up in the user's checkout.
type EnvDisk struct{}

var _ EnvFiles = EnvDisk{}

// Copy provisions the files of a new worktree from the source its strategy
// names; a file with no source is skipped and reported, never an error.
func (EnvDisk) Copy(ctx context.Context, request EnvCopy) ([]EnvWarning, error) {
	sources, warnings, err := CopySources(CopySourcesParams{Request: request, Exists: fs.FileExists})
	if err != nil {
		return nil, err
	}
	for _, source := range sources {
		if err := copyEnv(ctx, source); err != nil {
			return nil, err
		}
	}
	return warnings, nil
}

// Edit reads the file under its lock and writes what edit makes of it, so an
// edit computed from the lines never overwrites a change made since. A file
// that renders as it was is not rewritten.
func (EnvDisk) Edit(ctx context.Context, edit EnvEdit) (bool, error) {
	changed := false
	_, err := fs.UpdateFile(ctx, fs.UpdateFileParams{
		Path:     edit.Path,
		LockPath: fs.ScratchLockPath(edit.Path),
		Mutate: func(current []byte) (fs.Change, error) {
			next, write := EditEnv(EditEnvParams{Edit: edit, Current: current})
			if !write {
				return fs.Keep{}, nil
			}
			changed = true
			return fs.Write{Data: next}, nil
		},
	})
	if err != nil {
		return false, err
	}
	return changed, nil
}

type EditEnvParams struct {
	Edit EnvEdit
	// Current is the file's content, nil when it does not exist.
	Current []byte
}

// EditEnv is what an Edit writes over Current, and whether it writes at all.
func EditEnv(params EditEnvParams) ([]byte, bool) {
	if params.Current == nil && !params.Edit.Create {
		return nil, false
	}
	var lines []domain.EnvLine
	if params.Current != nil {
		lines = rules.ParseEnv(string(params.Current))
	}
	rendered := rules.RenderEnv(params.Edit.Edit(lines))
	if params.Current != nil && rendered == rules.RenderEnv(lines) {
		return nil, false
	}
	return []byte(rendered), true
}

func copyEnv(ctx context.Context, source EnvSource) error {
	data, err := os.ReadFile(source.Src)
	if err != nil {
		return fmt.Errorf("read %s: %w", source.Src, err)
	}
	_, err = fs.UpdateFile(ctx, fs.UpdateFileParams{
		Path:     source.Dst,
		LockPath: fs.ScratchLockPath(source.Dst),
		Mutate:   func([]byte) (fs.Change, error) { return fs.Write{Data: data}, nil },
	})
	return err
}
