package fs

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const defaultPerm os.FileMode = 0o644

var (
	// ErrRemoveFile, returned by Mutate, deletes the file instead of writing it,
	// under the same lock.
	ErrRemoveFile = errors.New("remove the file")
	// ErrUnchanged, returned by Mutate, leaves the file as it is, absent included.
	ErrUnchanged = errors.New("leave the file as it is")
)

type UpdateFileParams struct {
	Path string
	// LockPath is the lock file, <Path>.lock when empty. A file in the user's
	// tree takes ScratchLockPath, so no lock file appears in their checkout.
	LockPath string
	// Perm applies to a file this update creates; an existing file keeps its mode.
	Perm os.FileMode
	// Mutate receives the file's content, nil when it does not exist, and
	// returns the content to write.
	Mutate func(current []byte) ([]byte, error)
}

// UpdateFile is the one way wtm writes a file it keeps: it locks <path>.lock,
// reads the file, hands it to Mutate, then writes the result to a temporary
// file that is synced and renamed over the original, so a reader sees the old
// content or the new one, never half of it, and two writers never lose each
// other's change. Content Mutate leaves unchanged is not rewritten. ctx bounds
// only the wait for the lock: a free lock is taken even once ctx is done, so the
// record of work already carried out is not lost to an interrupt.
func UpdateFile(ctx context.Context, params UpdateFileParams) (Lock, error) {
	if err := os.MkdirAll(filepath.Dir(params.Path), 0o755); err != nil {
		return Lock{}, fmt.Errorf("create %s: %w", filepath.Dir(params.Path), err)
	}
	return WithFileLock(ctx, WithFileLockParams{
		Path: lockPathOf(params),
		Do:   func() error { return update(params) },
	})
}

const (
	lockSuffix     = ".lock"
	scratchLockDir = "wtm-locks"
)

func lockPathOf(params UpdateFileParams) string {
	if params.LockPath != "" {
		return params.LockPath
	}
	return params.Path + lockSuffix
}

// ScratchLockPath is the lock of a file wtm writes in the user's tree: in the
// temporary directory, named after the file's absolute path, so every process
// updating that file meets on the same lock.
func ScratchLockPath(path string) string {
	absolute, err := filepath.Abs(path)
	if err != nil {
		absolute = path
	}
	sum := sha256.Sum256([]byte(absolute))
	return filepath.Join(os.TempDir(), scratchLockDir, hex.EncodeToString(sum[:16])+lockSuffix)
}

func update(params UpdateFileParams) error {
	current, info, err := read(params.Path)
	if err != nil {
		return err
	}
	next, err := params.Mutate(current)
	if errors.Is(err, ErrRemoveFile) {
		return remove(params.Path)
	}
	if errors.Is(err, ErrUnchanged) {
		return nil
	}
	if err != nil {
		return err
	}
	if info != nil && bytes.Equal(current, next) {
		return nil
	}
	return replace(replaceParams{Path: params.Path, Data: next, Perm: permOf(info, params.Perm)})
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

func permOf(info os.FileInfo, perm os.FileMode) os.FileMode {
	if info != nil {
		return info.Mode().Perm()
	}
	if perm == 0 {
		return defaultPerm
	}
	return perm
}

func remove(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

type ReplaceFileParams struct {
	Path string
	Data []byte
	Perm os.FileMode
}

// ReplaceFile writes Data atomically without a lock, for a file one process
// alone writes by construction (the daemon's job index, under its own lock).
// Every other file goes through UpdateFile.
func ReplaceFile(params ReplaceFileParams) error {
	if err := os.MkdirAll(filepath.Dir(params.Path), 0o755); err != nil {
		return fmt.Errorf("create %s: %w", filepath.Dir(params.Path), err)
	}
	return replace(replaceParams{Path: params.Path, Data: params.Data, Perm: permOf(nil, params.Perm)})
}

type replaceParams struct {
	Path string
	Data []byte
	Perm os.FileMode
}

func replace(params replaceParams) error {
	dir := filepath.Dir(params.Path)
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(params.Path)+".*")
	if err != nil {
		return fmt.Errorf("write %s: %w", params.Path, err)
	}
	defer os.Remove(tmp.Name())

	if err := writeSynced(tmp, params); err != nil {
		tmp.Close()
		return fmt.Errorf("write %s: %w", params.Path, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("write %s: %w", params.Path, err)
	}
	if err := os.Rename(tmp.Name(), params.Path); err != nil {
		return fmt.Errorf("write %s: %w", params.Path, err)
	}
	syncDir(dir)
	return nil
}

func writeSynced(tmp *os.File, params replaceParams) error {
	if _, err := tmp.Write(params.Data); err != nil {
		return err
	}
	if err := tmp.Chmod(params.Perm); err != nil {
		return err
	}
	return tmp.Sync()
}

// syncDir makes the rename itself durable. Some filesystems refuse to sync a
// directory; the file is already in place by then, so that is not worth failing.
func syncDir(dir string) {
	handle, err := os.Open(dir)
	if err != nil {
		return
	}
	defer handle.Close()
	_ = handle.Sync()
}
