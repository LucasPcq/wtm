package configset_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/next/internal/adapter/config"
	"github.com/LucasPcq/wtm/next/internal/command/configset"
	"github.com/LucasPcq/wtm/next/internal/service/operation"
	"github.com/LucasPcq/wtm/next/internal/service/operationtest"
	"github.com/LucasPcq/wtm/next/internal/service/settings"

	"github.com/stretchr/testify/require"
)

// stateDir is a state directory holding content as its config.toml, none when content is "".
func stateDir(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	if content != "" {
		require.NoError(t, os.WriteFile(filepath.Join(dir, "config.toml"), []byte(content), 0o644))
	}
	return dir
}

// read is the config.toml of dir.
func read(t *testing.T, dir string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, "config.toml"))
	require.NoError(t, err)
	return string(data)
}

// usecase is config set over the config.toml of dir, under ops.
func usecase(dir string, ops operation.Service) configset.UseCase {
	return configset.UseCase{Settings: settings.Service{Store: config.Project{StateDir: dir}}, Git: branches{}, Detect: detected, Operations: ops}
}

// operations is an operations service over in-memory locks and events.
func operations() operation.Service {
	locks := &operationtest.Locks{}
	return operation.Service{Locker: locks, Publisher: locks, NewOpID: func() string { return "op-1" }, Now: time.Now}
}

// answered is the request with each path answered by its values.
func answered(t *testing.T, answers map[string][]string) configset.Request {
	t.Helper()
	req := configset.Request{}
	for path, values := range answers {
		var err error
		req, err = req.Answer(path, values)
		require.NoError(t, err)
	}
	return req
}
