package configshow_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/adapter/config"
	"github.com/LucasPcq/wtm/next/internal/command/configshow"
	"github.com/LucasPcq/wtm/next/internal/service/settings"

	"github.com/stretchr/testify/require"
)

// usecase is config show over a state directory holding content as its config.toml, none when content is "".
func usecase(t *testing.T, content string) configshow.UseCase {
	t.Helper()
	dir := t.TempDir()
	if content != "" {
		require.NoError(t, os.WriteFile(filepath.Join(dir, "config.toml"), []byte(content), 0o644))
	}
	return configshow.UseCase{Settings: settings.Service{Store: config.Project{StateDir: dir}}, Git: branches{}}
}
