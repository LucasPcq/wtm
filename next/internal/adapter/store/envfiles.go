package store

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/LucasPcq/wtm/next/internal/adapter/statefile"
	"github.com/LucasPcq/wtm/next/internal/wtm"
	"github.com/LucasPcq/wtm/next/internal/wtm/dotenv"
	"github.com/LucasPcq/wtm/next/internal/wtm/envcopy"
)

type EnvFiles struct{}

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

func (EnvFiles) Copy(ctx context.Context, request envcopy.Request) (envcopy.Copied, error) {
	candidates, err := envcopy.Candidates(request)
	if err != nil {
		return envcopy.Copied{}, err
	}
	present, err := presentOf(candidates)
	if err != nil {
		return envcopy.Copied{}, err
	}
	sources, warnings, err := envcopy.Sources(envcopy.SourcesParams{Request: request, Present: present})
	if err != nil {
		return envcopy.Copied{}, err
	}
	copied := envcopy.Copied{Warnings: warnings}
	for _, source := range sources {
		written, err := copyFile(ctx, source)
		if err != nil {
			return envcopy.Copied{}, err
		}
		if written.Changed {
			copied.Files = append(copied.Files, source.To)
		}
		copied.Unlocked = copied.Unlocked || written.Unlocked
	}
	return copied, nil
}

func (EnvFiles) Edit(ctx context.Context, edit dotenv.Edit) (wtm.Written, error) {
	changed := false
	lock, err := statefile.Update(ctx, statefile.UpdateParams{
		Path:     edit.Path,
		LockPath: statefile.ScratchLockPath(edit.Path),
		Mutate: func(current []byte) (statefile.Change, error) {
			next, write := edit.Over(current)
			if !write {
				return statefile.Keep{}, nil
			}
			changed = true
			return statefile.Write{Data: next}, nil
		},
	})
	if err != nil {
		return wtm.Written{}, err
	}
	return wtm.Written{Changed: changed, Unlocked: !lock.Held}, nil
}

func presentOf(paths []string) (map[string]bool, error) {
	present := map[string]bool{}
	for _, path := range paths {
		_, err := os.Stat(path)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
		present[path] = err == nil
	}
	return present, nil
}

func copyFile(ctx context.Context, source envcopy.Source) (wtm.Written, error) {
	data, err := os.ReadFile(source.From)
	if err != nil {
		return wtm.Written{}, fmt.Errorf("read %s: %w", source.From, err)
	}
	changed := false
	lock, err := statefile.Update(ctx, statefile.UpdateParams{
		Path:     source.To,
		LockPath: statefile.ScratchLockPath(source.To),
		Mutate: func(current []byte) (statefile.Change, error) {
			if current != nil && bytes.Equal(current, data) {
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
