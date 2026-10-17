package oplock

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/LucasPcq/wtm/next/internal/adapter/statefile"
	"github.com/LucasPcq/wtm/next/internal/domain"
)

// heldTempPrefix marks a record still being written, which running skips.
const heldTempPrefix = "."

// A holder is the record of a running operation: a JSON file kept
// exclusively flocked by its process for as long as it runs, so a record whose
// lock can be taken belongs to a process that is gone.
type holder struct {
	path string
	file *os.File
}

// hold writes the record of op at path and keeps it locked.
func hold(path string, op domain.Operation) (holder, error) {
	data, err := json.Marshal(op)
	if err != nil {
		return holder{}, err
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return holder{}, err
	}

	// Written and locked before it appears under its name: a reader never finds it half written or unlocked.
	file, err := os.CreateTemp(dir, heldTempPrefix+filepath.Base(path)+"-*")
	if err != nil {
		return holder{}, err
	}
	_ = statefile.Flock(file, syscall.LOCK_EX|syscall.LOCK_NB)
	if _, err := file.Write(data); err != nil {
		return holder{}, discard(file, err)
	}
	if err := os.Rename(file.Name(), path); err != nil {
		return holder{}, discard(file, err)
	}

	return holder{path: path, file: file}, nil
}

// rehold rewrites the record as op, keeping the old one if the new one cannot
// be written.
func (h holder) rehold(op domain.Operation) holder {
	next, err := hold(h.path, op)
	if err != nil {
		return h
	}

	closeAll([]*os.File{h.file})
	return next
}

// release removes the record, then lets its lock go.
func (h holder) release() {
	if h.file == nil {
		return
	}

	_ = os.Remove(h.path)
	closeAll([]*os.File{h.file})
}

// discard closes and removes a record that could not be put in place, and
// returns cause with any error of the removal.
func discard(file *os.File, cause error) error {
	file.Close()
	return errors.Join(cause, os.Remove(file.Name()))
}

// running returns the operations whose record in dir is held by a live
// process; records left by a dead one, or that do not decode, are skipped.
func running(dir string) []domain.Operation {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var found []domain.Operation
	for _, entry := range entries {
		if entry.IsDir() || strings.HasPrefix(entry.Name(), heldTempPrefix) {
			continue
		}

		data, live := readHeld(filepath.Join(dir, entry.Name()))
		var op domain.Operation
		if live && json.Unmarshal(data, &op) == nil {
			found = append(found, op)
		}
	}
	return found
}

// readHeld returns the record at path and reports whether its process still
// holds it: a shared lock that can be taken means none does.
func readHeld(path string) ([]byte, bool) {
	file, err := os.Open(path)
	if err != nil {
		return nil, false
	}
	defer file.Close()

	if statefile.Flock(file, syscall.LOCK_SH|syscall.LOCK_NB) == nil {
		return nil, false
	}

	data, err := io.ReadAll(file)
	return data, err == nil
}
