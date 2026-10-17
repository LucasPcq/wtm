package oplock

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/LucasPcq/wtm/next/internal/domain"

	"github.com/LucasPcq/wtm/next/internal/adapter/statefile"
)

const heldTempPrefix = "."

type holder struct {
	path string
	file *os.File
}

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

func (h holder) rehold(op domain.Operation) holder {
	next, err := hold(h.path, op)
	if err != nil {
		return h
	}
	closeAll([]*os.File{h.file})
	return next
}

func (h holder) release() {
	if h.file == nil {
		return
	}
	_ = os.Remove(h.path)
	closeAll([]*os.File{h.file})
}

func discard(file *os.File, cause error) error {
	file.Close()
	return errors.Join(cause, os.Remove(file.Name()))
}

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
