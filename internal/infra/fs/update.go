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

// Change is what Mutate decides for the file: Write, Keep or Remove.
type Change interface{ change() }

// Write replaces the file's content; content equal to what is there is not rewritten.
type Write struct{ Data []byte }

// Keep leaves the file as it is, an absent one included.
type Keep struct{}

// Remove deletes the file, under the same lock.
type Remove struct{}

func (Write) change()  {}
func (Keep) change()   {}
func (Remove) change() {}

var errNoChange = errors.New("update decided no change: Mutate returned a nil Change")

type UpdateFileParams struct {
	Path string
	// LockPath is the lock file, <Path>.lock when empty. A file in the user's
	// tree takes ScratchLockPath, so no lock file appears in their checkout.
	LockPath string
	// Perm applies to a file this update creates; an existing file keeps its mode.
	Perm os.FileMode
	// Mutate receives the file's content, nil when it does not exist.
	Mutate func(current []byte) (Change, error)
}

// UpdateFile is the one way wtm writes a file it keeps: it locks the file,
// reads it, hands it to Mutate and writes what Mutate decides atomically, so a
// reader sees the old content or the new one, never half of it, and two
// writers never lose each other's change. ctx bounds only the wait for the
// lock: a free lock is taken even once ctx is done, so the record of work
// already carried out is not lost to an interrupt.
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
