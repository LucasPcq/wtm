package fs

import "os"

// RemoveDir drops a directory wtm keeps and everything in it; an absent one is
// already removed.
func RemoveDir(path string) error {
	return os.RemoveAll(path)
}
