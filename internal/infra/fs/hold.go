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

// HoldFile writes Data at Path and keeps it locked until Release, which
// removes it: the file says who holds something for exactly as long as they
// hold it, and a process that dies lets go of it with its descriptors. The
// data is written and locked before the file appears under Path, so a reader
// never finds it half written or unlocked. Where flock is unavailable the file
// is still written and removed, and Held is false.
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

func discard(file *os.File, cause error) error {
	file.Close()
	return errors.Join(cause, os.Remove(file.Name()))
}

// HeldFiles reads every file of dir still held through HoldFile. A file nobody
// holds was left by a process that died, and is removed. Where flock is
// unavailable, whether a file is held cannot be told and every one is read.
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

// removeIfSame removes the stale file only if path still names it, never a
// file renamed over it since it was opened.
func removeIfSame(file *os.File, path string) {
	opened, err := file.Stat()
	if err != nil {
		return
	}
	current, err := os.Stat(path)
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
