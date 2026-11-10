package shell

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Legacy ran $EDITOR as one path, so "code --wait" failed: an editor is a word list.
func TestAnEditorWithArgumentsOpensTheFile(t *testing.T) {
	dir := t.TempDir()
	editor := writeEditor(t, dir, `printf '%s %s' "$1" "$2" > "$2.args"`)
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", editor+" --wait")
	file := filepath.Join(dir, "config toml")

	require.NoError(t, Edit(t.Context(), file))

	args, err := os.ReadFile(file + ".args")
	require.NoError(t, err)
	assert.Equal(t, "--wait "+file, string(args))
}

// Legacy ignored $VISUAL.
func TestVisualComesBeforeEditor(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("VISUAL", writeEditor(t, dir, `touch "$1.visual" # visual`))
	t.Setenv("EDITOR", writeEditor(t, dir, `touch "$1.editor"`))
	file := filepath.Join(dir, "config.toml")

	require.NoError(t, Edit(t.Context(), file))

	assert.FileExists(t, file+".visual")
	assert.NoFileExists(t, file+".editor")
}

func TestAnEditorThatFailsIsAnError(t *testing.T) {
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", writeEditor(t, t.TempDir(), "exit 3"))

	assert.Error(t, Edit(t.Context(), "config.toml"))
}

// Legacy launched vi with no terminal to draw on.
func TestNoEditorAndNoTerminalIsRefused(t *testing.T) {
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "")
	stdin := os.Stdin
	devnull, err := os.Open(os.DevNull)
	require.NoError(t, err)
	t.Cleanup(func() { os.Stdin = stdin; _ = devnull.Close() })
	os.Stdin = devnull

	assert.ErrorIs(t, Edit(t.Context(), "config.toml"), ErrNoEditor)
}
