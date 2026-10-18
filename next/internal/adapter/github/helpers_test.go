package github_test

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// listed is gh's list of n open pull requests, on branches b1 to bn.
func listed(n int) string {
	prs := make([]string, 0, n)
	for i := range n {
		prs = append(prs, `{"number": `+strconv.Itoa(i+1)+`, "headRefName": "b`+strconv.Itoa(i+1)+`", "url": "u"}`)
	}
	return "[" + strings.Join(prs, ",") + "]"
}

// fakeGH writes a gh that answers auth status with authExit and pr list with prs, and returns its path.
func fakeGH(t *testing.T, authExit int, prs string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "gh")
	script := "#!/bin/sh\nif [ \"$1\" = auth ]; then exit " + strconv.Itoa(authExit) + "; fi\ncat <<'JSON'\n" + prs + "\nJSON\n"
	require.NoError(t, os.WriteFile(path, []byte(script), 0o755))
	return path
}
