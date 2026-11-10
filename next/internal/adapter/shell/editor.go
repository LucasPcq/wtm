package shell

import (
	"cmp"
	"context"
	"errors"
	"os"
	"os/exec"

	"golang.org/x/term"
)

// fallbackEditor opens a file when neither $VISUAL nor $EDITOR names an editor, as git and crontab do.
const fallbackEditor = "vi"

// ErrNoEditor refuses to open a file with no editor named and no terminal for the fallback to run in.
var ErrNoEditor = errors.New("no editor: set $VISUAL or $EDITOR, or run it in a terminal")

// Edit opens path in the user's editor, $VISUAL else $EDITOR else vi, attached to this process's terminal, and returns once it is closed. The editor is a shell word list ("code --wait"), path its last argument. It runs in the terminal's foreground group, not a group of its own, since an editor reads the terminal; Ctrl+C reaches it from there, so a cancelled ctx leaves it be.
func Edit(_ context.Context, path string) error {
	editor := cmp.Or(os.Getenv("VISUAL"), os.Getenv("EDITOR"))
	if editor == "" && !terminal(os.Stdin) {
		return ErrNoEditor
	}

	// path travels as $1, never through the shell's parsing
	cmd := exec.Command("sh", "-c", cmp.Or(editor, fallbackEditor)+` "$1"`, "sh", path)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return cmd.Run()
}

// terminal reports whether f is a terminal; /dev/null is a character device, not one.
func terminal(f *os.File) bool {
	return term.IsTerminal(int(f.Fd()))
}
