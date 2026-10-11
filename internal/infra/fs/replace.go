package fs

import (
	"fmt"
	"os"
	"path/filepath"
)

const defaultPerm os.FileMode = 0o644

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

func permOf(info os.FileInfo, perm os.FileMode) os.FileMode {
	if info != nil {
		return info.Mode().Perm()
	}
	if perm == 0 {
		return defaultPerm
	}
	return perm
}

type replaceParams struct {
	Path string
	Data []byte
	Perm os.FileMode
}

// replace writes beside the file, syncs, then renames over it: the rename is
// what makes the write all or nothing.
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
