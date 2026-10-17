package store

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/LucasPcq/wtm/next/internal/adapter/statefile"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/dotenv"
	"github.com/LucasPcq/wtm/next/internal/domain/env"
)

// ownerOnly is the mode of a .env file wtm creates: it may hold secrets.
const ownerOnly os.FileMode = 0o600

// EnvFiles reads, copies and edits the .env files of worktrees on disk. Its
// writes go through statefile, under a lock kept outside the worktree.
type EnvFiles struct{}

// Read returns the lines of the .env file at path, none when it does not
// exist, or an error naming the path when it cannot be read.
func (EnvFiles) Read(path string) ([]dotenv.Line, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	return dotenv.Parse(data), nil
}

// Copy copies the .env files request names into the new worktree, each with
// the mode of its source, and returns the files it changed, a warning for each
// file copied from a fallback or not at all, and whether a copy ran without its
// lock. It stops at the first file it cannot read or write.
func (EnvFiles) Copy(ctx context.Context, request env.CopyRequest) (env.Copied, error) {
	sources, warnings, err := env.Sources(request, exists)
	if err != nil {
		return env.Copied{}, err
	}

	copied := env.Copied{Warnings: warnings}
	for _, source := range sources {
		written, err := copyFile(ctx, source)
		if err != nil {
			return env.Copied{}, err
		}
		if written.Changed {
			copied.Files = append(copied.Files, source.To)
		}
		copied.Unlocked = copied.Unlocked || written.Unlocked
	}
	return copied, nil
}

// Edit applies edit to the .env file at edit.Path, creating it owner-only, and
// returns what it wrote; an edit that changes nothing leaves the file alone.
func (EnvFiles) Edit(ctx context.Context, edit dotenv.Edit) (domain.Written, error) {
	return statefile.Update(ctx, statefile.UpdateParams{
		Path:     edit.Path,
		LockPath: statefile.ScratchLockPath(edit.Path),
		Perm:     ownerOnly,
		Mutate: func(current []byte) (statefile.Change, error) {
			next, write := edit.Over(current)
			if !write {
				return statefile.Keep{}, nil
			}
			return statefile.Write{Data: next}, nil
		},
	})
}

// exists reports whether a file exists at path; an error other than its
// absence is returned.
func exists(path string) (bool, error) {
	_, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return err == nil, err
}

// copyFile writes the content of source.From to source.To with the same mode.
func copyFile(ctx context.Context, source env.Source) (domain.Written, error) {
	data, err := os.ReadFile(source.From)
	if err != nil {
		return domain.Written{}, fmt.Errorf("read %s: %w", source.From, err)
	}

	info, err := os.Stat(source.From)
	if err != nil {
		return domain.Written{}, fmt.Errorf("read %s: %w", source.From, err)
	}

	return statefile.Update(ctx, statefile.UpdateParams{
		Path:     source.To,
		LockPath: statefile.ScratchLockPath(source.To),
		Perm:     info.Mode().Perm(),
		Mutate:   func([]byte) (statefile.Change, error) { return statefile.Write{Data: data}, nil },
	})
}
