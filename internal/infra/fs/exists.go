package fs

import (
	"errors"
	"os"
)

// Exists is false only for a path that is not there: a path that cannot be
// stat'ed for another reason is an error, never taken for absent.
func Exists(path string) (bool, error) {
	_, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// FileExists takes any error for absence, for the commands still on flow/.
func FileExists(path string) bool {
	exists, err := Exists(path)
	return exists && err == nil
}
