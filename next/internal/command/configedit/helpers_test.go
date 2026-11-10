package configedit_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/adapter/config"
	"github.com/LucasPcq/wtm/next/internal/command/configedit"
	"github.com/LucasPcq/wtm/next/internal/service/settings"

	"github.com/stretchr/testify/require"
)

// usecase is config edit over a state directory holding content as its config.toml (none when ""), its editor writing edited in its place (removing it when ""), or failing with fail.
func usecase(t *testing.T, content, edited string, fail error) configedit.UseCase {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if content != "" {
		require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	}
	editor := func(_ context.Context, at string) error {
		if fail != nil {
			return fail
		}
		if edited == "" {
			return os.Remove(at)
		}
		return os.WriteFile(at, []byte(edited), 0o644)
	}
	return configedit.UseCase{Path: path, Editor: editor, Settings: settings.Service{Store: config.Project{StateDir: dir}}, Git: branches{}}
}
