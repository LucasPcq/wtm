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

type Change interface{ change() }

type Write struct{ Data []byte }

type Keep struct{}

func (Write) change() {}
func (Keep) change()  {}

const lockSuffix = ".lock"

var errNoChange = errors.New("update decided no change: Mutate returned a nil Change")

type UpdateParams struct {
	Path     string
	LockPath string
	Perm     os.FileMode
	Mutate   func(current []byte) (Change, error)
}

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
