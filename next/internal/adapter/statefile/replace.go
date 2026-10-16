package statefile

import (
	"fmt"
	"os"
	"path/filepath"
)

const defaultPerm os.FileMode = 0o644

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

func syncDir(dir string) {
	handle, err := os.Open(dir)
	if err != nil {
		return
	}
	defer handle.Close()
	// Some filesystems refuse to sync a directory; the rename has landed already.
	_ = handle.Sync()
}
