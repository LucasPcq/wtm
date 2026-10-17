package oplock

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/stretchr/testify/require"
)

// take returns the locks taken, failing the test on an error, and releases
// them when the test ends.
func take(t *testing.T, taken func() (*Held, error)) *Held {
	t.Helper()
	held, err := taken()
	require.NoError(t, err)
	t.Cleanup(held.Release)
	return held
}

// refusal returns err as the *domain.Error it must be.
func refusal(t *testing.T, err error) *domain.Error {
	t.Helper()
	failure, ok := errors.AsType[*domain.Error](err)
	require.True(t, ok, err)
	return failure
}

func read(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return data
}

func mkdir(t *testing.T, dir string) string {
	t.Helper()
	require.NoError(t, os.MkdirAll(dir, 0o755))
	return dir
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	mkdir(t, filepath.Dir(path))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

// flockExclusive takes file's exclusive lock without waiting, as another
// process would, bypassing statefile.
func flockExclusive(file *os.File) error {
	return syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
}
