package fs

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

const heldTempPrefix = "."

type HoldFileParams struct {
	Path string
	Data []byte
}

// HoldFile keeps Data at Path, locked, until Release removes it: a process that
// dies lets go of it with its descriptors. The file is written and locked
// before it appears, so a reader never finds it half written or unlocked.
func HoldFile(params HoldFileParams) (Locks, error) {
	dir := filepath.Dir(params.Path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return Locks{}, err
	}
	file, err := os.CreateTemp(dir, heldTempPrefix+filepath.Base(params.Path)+"-*")
	if err != nil {
		return Locks{}, err
	}
	held := flock(file, syscall.LOCK_EX|syscall.LOCK_NB) == nil
	if _, err := file.Write(params.Data); err != nil {
		return Locks{}, discard(file, err)
	}
	if err := os.Rename(file.Name(), params.Path); err != nil {
		return Locks{}, discard(file, err)
	}
	return Locks{Held: held, files: []*os.File{file}, remove: params.Path}, nil
}

type ReholdParams struct {
	Held Locks
	Data []byte
}

// Rehold puts Data in place of a file HoldFile holds, with no moment the path
// holds nothing: the new file is renamed over the old before its lock goes.
func Rehold(params ReholdParams) (Locks, error) {
	next, err := HoldFile(HoldFileParams{Path: params.Held.remove, Data: params.Data})
	if err != nil {
		return params.Held, err
	}
	params.Held.remove = ""
	params.Held.Release()
	return next, nil
}

func discard(file *os.File, cause error) error {
	file.Close()
	return errors.Join(cause, os.Remove(file.Name()))
}

// HeldFiles reads the files of dir still held; one nobody holds was left by a
// dead process and is removed. Without flock every file is read.
func HeldFiles(dir string) ([][]byte, error) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var held [][]byte
	for _, entry := range entries {
		if entry.IsDir() || strings.HasPrefix(entry.Name(), heldTempPrefix) {
			continue
		}
		data, live, err := readHeld(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, err
		}
		if live {
			held = append(held, data)
		}
	}
	return held, nil
}

func removeIfSame(file *os.File, path string) {
	opened, err := file.Stat()
	if err != nil {
		return
	}
	current, err := os.Stat(path)
	// A file renamed over the stale one since it was opened is not the stale one.
	if err != nil || !os.SameFile(opened, current) {
		return
	}
	_ = os.Remove(path)
}

func readHeld(path string) ([]byte, bool, error) {
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	defer file.Close()

	err = flock(file, syscall.LOCK_SH|syscall.LOCK_NB)
	if err == nil {
		removeIfSame(file, path)
		_ = flock(file, syscall.LOCK_UN)
		return nil, false, nil
	}
	data, readErr := io.ReadAll(file)
	return data, true, readErr
}
