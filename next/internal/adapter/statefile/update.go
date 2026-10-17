// Package statefile writes the files wtm keeps: under a flock, atomically, refusing one a newer wtm wrote.
package statefile

import (
	"bytes"
	"cmp"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/LucasPcq/wtm/next/internal/domain"
)

//go-sumtype:decl Change

// A Change is what an update decided to do with a file: [Write] or [Keep].
type Change interface{ change() }

// Write replaces the file with Data.
type Write struct{ Data []byte }

// Keep leaves the file as it is.
type Keep struct{}

func (Write) change() {}
func (Keep) change()  {}

const lockSuffix = ".lock"

var errNoChange = errors.New("update decided no change: Mutate returned a nil Change")

// UpdateParams is the file an [Update] changes and how. LockPath defaults to
// Path with ".lock" appended, Perm to the permissions of the file in place,
// else 0644. Mutate receives the file's content, nil when it does not exist,
// and decides the [Change].
type UpdateParams struct {
	Path     string
	LockPath string
	Perm     os.FileMode
	Mutate   func(current []byte) (Change, error)
}

// Update reads, decides and writes the file at params.Path under its lock, so
// two processes never lose each other's change, and replaces it atomically. A
// Write of the content already there changes nothing. It returns what it did;
// where the lock cannot be taken it writes unlocked, and says so. It returns
// Mutate's error, an I/O error naming the path, ctx's error when cancelled
// while waiting for the lock, or an error when Mutate returns a nil Change.
func Update(ctx context.Context, params UpdateParams) (domain.Written, error) {
	if err := os.MkdirAll(filepath.Dir(params.Path), 0o755); err != nil {
		return domain.Written{}, fmt.Errorf("create %s: %w", filepath.Dir(params.Path), err)
	}

	lockPath := cmp.Or(params.LockPath, params.Path+lockSuffix)
	changed := false
	held, err := WithLock(ctx, lockPath, func() (err error) {
		changed, err = update(params)
		return err
	})
	return domain.Written{Changed: changed, Unlocked: !held}, err
}

// update runs one read-decide-write of params.Path, the lock held, and
// reports whether it changed the file.
func update(params UpdateParams) (bool, error) {
	current, info, err := read(params.Path)
	if err != nil {
		return false, err
	}

	change, err := params.Mutate(current)
	if err != nil {
		return false, err
	}

	switch next := change.(type) {
	case Keep:
		return false, nil
	case Write:
		if info != nil && bytes.Equal(current, next.Data) {
			return false, nil
		}
		return true, replace(params.Path, next.Data, permOf(info, params.Perm))
	}
	return false, errNoChange
}

// read returns the content of the file at path and its info, both nil when it
// does not exist.
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
