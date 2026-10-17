package oplock

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/wtm"
	"github.com/stretchr/testify/require"
)

func take(t *testing.T, taken func() (*Held, error)) *Held {
	t.Helper()
	held, err := taken()
	require.NoError(t, err)
	t.Cleanup(held.Release)
	return held
}

func refusal(t *testing.T, err error) *wtm.Error {
	t.Helper()
	failure, ok := errors.AsType[*wtm.Error](err)
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

func flockExclusive(file *os.File) error {
	return syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
}
