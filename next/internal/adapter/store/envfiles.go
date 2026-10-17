package store

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/LucasPcq/wtm/next/internal/wtm/env"

	"github.com/LucasPcq/wtm/next/internal/adapter/statefile"
	"github.com/LucasPcq/wtm/next/internal/wtm"
	"github.com/LucasPcq/wtm/next/internal/wtm/dotenv"
)

const ownerOnly os.FileMode = 0o600

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

func (EnvFiles) Copy(ctx context.Context, request env.CopyRequest) (env.Copied, error) {
	sources, warnings, err := env.Sources(env.SourcesParams{Request: request, Exists: exists})
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

func (EnvFiles) Edit(ctx context.Context, edit dotenv.Edit) (wtm.Written, error) {
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

func exists(path string) (bool, error) {
	_, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return err == nil, err
}

func copyFile(ctx context.Context, source env.Source) (wtm.Written, error) {
	data, err := os.ReadFile(source.From)
	if err != nil {
		return wtm.Written{}, fmt.Errorf("read %s: %w", source.From, err)
	}
	info, err := os.Stat(source.From)
	if err != nil {
		return wtm.Written{}, fmt.Errorf("read %s: %w", source.From, err)
	}
	return statefile.Update(ctx, statefile.UpdateParams{
		Path:     source.To,
		LockPath: statefile.ScratchLockPath(source.To),
		Perm:     info.Mode().Perm(),
		Mutate:   func([]byte) (statefile.Change, error) { return statefile.Write{Data: data}, nil },
	})
}
