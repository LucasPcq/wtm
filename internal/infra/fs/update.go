package fs

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

//go-sumtype:decl Change

type Change interface{ change() }

type Write struct{ Data []byte }

type Keep struct{}

type Remove struct{}

func (Write) change()  {}
func (Keep) change()   {}
func (Remove) change() {}

var errNoChange = errors.New("update decided no change: Mutate returned a nil Change")

type UpdateFileParams struct {
	Path     string
	LockPath string
	Perm     os.FileMode
	Mutate   func(current []byte) (Change, error)
}

func UpdateFile(ctx context.Context, params UpdateFileParams) (Lock, error) {
	if err := os.MkdirAll(filepath.Dir(params.Path), 0o755); err != nil {
		return Lock{}, fmt.Errorf("create %s: %w", filepath.Dir(params.Path), err)
	}
	return WithFileLock(ctx, WithFileLockParams{
		Path: lockPathOf(params),
		Do:   func() error { return update(params) },
	})
}

func lockPathOf(params UpdateFileParams) string {
	if params.LockPath != "" {
		return params.LockPath
	}
	return params.Path + lockSuffix
}

func update(params UpdateFileParams) error {
	current, info, err := read(params.Path)
	if err != nil {
		return err
	}
	change, err := params.Mutate(current)
	if err != nil {
		return err
	}
	switch next := change.(type) {
	case Keep:
		return nil
	case Remove:
		return remove(params.Path)
	case Write:
		if info != nil && bytes.Equal(current, next.Data) {
			return nil
		}
		return replace(replaceParams{Path: params.Path, Data: next.Data, Perm: permOf(info, params.Perm)})
	}
	return errNoChange
}

func read(path string) ([]byte, os.FileInfo, error) {
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	return data, info, nil
}

func remove(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func RemoveDir(path string) error {
	return os.RemoveAll(path)
}
