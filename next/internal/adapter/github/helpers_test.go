package github_test

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

// fakeGH writes a gh that answers auth status with authExit and pr list with prs, and returns its path.
func fakeGH(t *testing.T, authExit int, prs string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "gh")
	script := "#!/bin/sh\nif [ \"$1\" = auth ]; then exit " + strconv.Itoa(authExit) + "; fi\ncat <<'JSON'\n" + prs + "\nJSON\n"
	require.NoError(t, os.WriteFile(path, []byte(script), 0o755))
	return path
}
