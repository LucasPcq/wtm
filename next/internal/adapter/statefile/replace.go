package statefile

import (
	"fmt"
	"os"
	"path/filepath"
)

const defaultPerm os.FileMode = 0o644

// permOf returns the permissions to write a file with: those of the file in
// place (info), else perm, else defaultPerm.
func permOf(info os.FileInfo, perm os.FileMode) os.FileMode {
	if info != nil {
		return info.Mode().Perm()
	}

	if perm == 0 {
		return defaultPerm
	}
	return perm
}

// replace writes data to path atomically: a reader sees the old file or the
// new one, never a part of it, even after a crash.
func replace(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".*")
	if err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	defer os.Remove(tmp.Name())

	if err := writeSynced(tmp, data, perm); err != nil {
		tmp.Close()
		return fmt.Errorf("write %s: %w", path, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}

	if err := os.Rename(tmp.Name(), path); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	syncDir(dir)
	return nil
}

// writeSynced writes data to tmp with perm and flushes it to disk.
func writeSynced(tmp *os.File, data []byte, perm os.FileMode) error {
	if _, err := tmp.Write(data); err != nil {
		return err
	}
	if err := tmp.Chmod(perm); err != nil {
		return err
	}
	return tmp.Sync()
}

// syncDir flushes dir to disk so a rename into it survives a crash.
func syncDir(dir string) {
	handle, err := os.Open(dir)
	if err != nil {
		return
	}
	defer handle.Close()
	// Some filesystems refuse to sync a directory; the rename has landed already.
	_ = handle.Sync()
}
