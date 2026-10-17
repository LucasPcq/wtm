package infra

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra/fs"
)

func RegistryPath() (string, error) {
	dir, err := fs.GlobalDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, domain.RegistryFileName), nil
}

func ReadRegistry() ([]domain.RegisteredRepo, error) {
	path, err := RegistryPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	repos, err := decodeRegistry(data)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return repos, nil
}

// UpdateRegistry is a read-modify-write of repos.json through fs.UpdateFile.
func UpdateRegistry(ctx context.Context, update func([]domain.RegisteredRepo) []domain.RegisteredRepo) error {
	path, err := RegistryPath()
	if err != nil {
		return err
	}
	_, err = fs.UpdateFile(ctx, fs.UpdateFileParams{
		Path: path,
		Mutate: func(current []byte) (fs.Change, error) {
			repos, err := decodeRegistry(current)
			// Unparsable is rebuilt from scratch: every repository rejoins the
			// next time a command runs in it.
			if err != nil && !errors.As(err, new(*json.SyntaxError)) && !errors.As(err, new(*json.UnmarshalTypeError)) {
				return nil, err
			}
			data, err := encodeRegistry(update(repos))
			if err != nil {
				return nil, err
			}
			return fs.Write{Data: data}, nil
		},
	})
	return err
}

func decodeRegistry(data []byte) ([]domain.RegisteredRepo, error) {
	if data == nil {
		return nil, nil
	}
	var repos []domain.RegisteredRepo
	if err := json.Unmarshal(data, &repos); err != nil {
		return nil, err
	}
	return repos, nil
}

func encodeRegistry(repos []domain.RegisteredRepo) ([]byte, error) {
	if repos == nil {
		repos = []domain.RegisteredRepo{}
	}
	data, err := json.MarshalIndent(repos, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}
