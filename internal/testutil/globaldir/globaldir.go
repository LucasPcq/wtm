// Package globaldir points a test's global wtm directory at a temporary one.
package globaldir

import (
	"os"
	"path/filepath"
	"testing"
)

// Isolate moves HOME — and with it the daemon socket and the global config —
// into a temporary directory. Without it a test that resolves a public proxy
// port reads whatever daemon the developer happens to be running, so the same
// code passes on one machine and fails on the next.
func Isolate(t testing.TB) {
	t.Helper()
	home := t.TempDir()
	// A short path: a unix socket's address is capped near 104 bytes, and
	// t.TempDir() under a long test name overruns it.
	short, err := os.MkdirTemp("", "wtmhome")
	if err == nil {
		t.Cleanup(func() { os.RemoveAll(short) })
		home = short
	}
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
}

// Main is Isolate for a whole package: a command test reaching LoadConfig
// registers its repository, and must not do it in the developer's registry.
func Main(m *testing.M) int {
	home, err := os.MkdirTemp("", "wtmhome")
	if err != nil {
		return 1
	}
	defer os.RemoveAll(home)
	os.Setenv("HOME", home)
	os.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	return m.Run()
}
