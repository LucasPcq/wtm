package infra

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/LucasPcq/wtm/internal/domain"
)

func RegistryPath() (string, error) {
	dir, err := GlobalDir()
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
	var repos []domain.RegisteredRepo
	if err := json.Unmarshal(data, &repos); err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return repos, nil
}

// UpdateRegistry is a read-modify-write under the registry's lock, written by
// rename so a reader never sees half a file.
func UpdateRegistry(update func([]domain.RegisteredRepo) []domain.RegisteredRepo) error {
	path, err := RegistryPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return WithFileLock(WithFileLockParams{
		Path: filepath.Join(filepath.Dir(path), domain.RegistryLockName),
		Do: func() error {
			repos, err := ReadRegistry()
			// Unparsable is rebuilt from scratch: every repository rejoins the
			// next time a command runs in it.
			if err != nil && !errors.As(err, new(*json.SyntaxError)) && !errors.As(err, new(*json.UnmarshalTypeError)) {
				return err
			}
			return writeRegistry(path, update(repos))
		},
	})
}

func writeRegistry(path string, repos []domain.RegisteredRepo) error {
	if repos == nil {
		repos = []domain.RegisteredRepo{}
	}
	data, err := json.MarshalIndent(repos, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), domain.RegistryFileName+".*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(append(data, '\n')); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
