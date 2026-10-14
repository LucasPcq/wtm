package store

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra/fs"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/rules/envcopy"
)

// EnvFiles are the .env files of a worktree, written under a lock kept outside the user's tree.
type EnvFiles interface {
	// Read is the file's lines; an absent file reads as none.
	Read(path string) ([]domain.EnvLine, error)
	// Template is the template provisioning copies for file from dir, "" when none is on disk.
	Template(dir string, file domain.EnvFile) (string, error)
	Copy(ctx context.Context, request envcopy.Request) (EnvCopied, error)
	Edit(ctx context.Context, edit EnvEdit) (Written, error)
}

// EnvCopied is what a copy did: the files it wrote, those it could not provision.
type EnvCopied struct {
	Files    []string
	Warnings []envcopy.Warning
	Unlocked bool
}

type EnvEdit struct {
	Path string
	// Create writes a file that does not exist; otherwise an absent file stays absent.
	Create bool
	// Edit receives the lines read under the lock, nil for an absent file.
	Edit func([]domain.EnvLine) []domain.EnvLine
}

// EnvDisk writes the .env files in place, each locked by fs.ScratchLockPath:
// a lock file beside it would show up in the user's checkout.
type EnvDisk struct{}

var _ EnvFiles = EnvDisk{}

func (EnvDisk) Read(path string) ([]domain.EnvLine, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return rules.ParseEnv(string(data)), nil
}

func (EnvDisk) Template(dir string, file domain.EnvFile) (string, error) {
	candidates := envcopy.TemplateCandidates(dir, file)
	present, err := presentOf(candidates)
	if err != nil {
		return "", err
	}
	return envcopy.Template(candidates, present), nil
}

// Copy provisions the files of a new worktree from the source its strategy
// names; a file with no source is skipped and reported, never an error.
func (EnvDisk) Copy(ctx context.Context, request envcopy.Request) (EnvCopied, error) {
	candidates, err := envcopy.Candidates(request)
	if err != nil {
		return EnvCopied{}, err
	}
	present, err := presentOf(candidates)
	if err != nil {
		return EnvCopied{}, err
	}
	sources, warnings, err := envcopy.Sources(envcopy.SourcesParams{Request: request, Present: present})
	if err != nil {
		return EnvCopied{}, err
	}
	copied := EnvCopied{Warnings: warnings}
	for _, source := range sources {
		written, err := copyEnv(ctx, source)
		if err != nil {
			return EnvCopied{}, err
		}
		if written.Changed {
			copied.Files = append(copied.Files, source.Dst)
		}
		copied.Unlocked = copied.Unlocked || written.Unlocked
	}
	return copied, nil
}

// Edit reads the file under its lock and writes what edit makes of it, so an
// edit computed from the lines never overwrites a change made since. A file
// that renders as it was is not rewritten.
func (EnvDisk) Edit(ctx context.Context, edit EnvEdit) (Written, error) {
	changed := false
	lock, err := fs.UpdateFile(ctx, fs.UpdateFileParams{
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
		return Written{}, err
	}
	return Written{Changed: changed, Unlocked: !lock.Held}, nil
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

func presentOf(paths []string) (map[string]bool, error) {
	present := map[string]bool{}
	for _, path := range paths {
		exists, err := fs.Exists(path)
		if err != nil {
			return nil, err
		}
		present[path] = exists
	}
	return present, nil
}

func copyEnv(ctx context.Context, source envcopy.Source) (Written, error) {
	data, err := os.ReadFile(source.Src)
	if err != nil {
		return Written{}, fmt.Errorf("read %s: %w", source.Src, err)
	}
	changed := false
	lock, err := fs.UpdateFile(ctx, fs.UpdateFileParams{
		Path:     source.Dst,
		LockPath: fs.ScratchLockPath(source.Dst),
		Mutate: func(current []byte) (fs.Change, error) {
			if current != nil && bytes.Equal(current, data) {
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
