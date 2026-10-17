package runctx

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/creack/pty"
)

// `run up > log` from a terminal: stdin is one, stdout is a file. The run
// module must not take the screen over for output nobody is watching.
func TestATerminalIsOwnedOnlyWhenBothEndsAreOne(t *testing.T) {
	ptmx, tty, err := pty.Open()
	if err != nil {
		t.Fatalf("open pty: %v", err)
	}
	t.Cleanup(func() { ptmx.Close(); tty.Close() })

	file, err := os.Create(filepath.Join(t.TempDir(), "log"))
	if err != nil {
		t.Fatalf("create log: %v", err)
	}
	t.Cleanup(func() { file.Close() })

	if !ownsTerminal(tty, tty) {
		t.Error("a pty on both ends was not taken for a terminal")
	}
	if ownsTerminal(tty, file) {
		t.Error("stdout redirected to a file was taken for a terminal")
	}
	if ownsTerminal(file, tty) {
		t.Error("stdin redirected from a file was taken for a terminal")
	}
}
