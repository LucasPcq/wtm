package github_test

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

// fakeGH writes a gh that answers auth status with authExit and any other call with answer, and returns its path.
func fakeGH(t *testing.T, authExit int, answer string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "gh")
	script := "#!/bin/sh\nif [ \"$1\" = auth ]; then exit " + strconv.Itoa(authExit) + "; fi\ncat <<'JSON'\n" + answer + "\nJSON\n"
	require.NoError(t, os.WriteFile(path, []byte(script), 0o755))
	return path
}

// partialGH writes a gh that answers auth status and prints answer then fails, as gh api graphql does when part of a query cannot be resolved, and returns its path.
func partialGH(t *testing.T, answer string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "gh")
	script := "#!/bin/sh\nif [ \"$1\" = auth ]; then exit 0; fi\ncat <<'JSON'\n" + answer + "\nJSON\necho 'gh: Resource not accessible by integration' >&2\nexit 1\n"
	require.NoError(t, os.WriteFile(path, []byte(script), 0o755))
	return path
}

// graphQLGH writes a gh that answers each GraphQL query with a merged pull request numbered 1 for each branch it asks of, its URL the branch's name, appends each call's branches to the log it returns, and fails the call that asks of failing.
func graphQLGH(t *testing.T, failing string) (string, string) {
	t.Helper()
	dir := t.TempDir()
	path, log := filepath.Join(dir, "gh"), filepath.Join(dir, "calls.log")
	script := `#!/bin/sh
if [ "$1" = auth ]; then exit 0; fi
printf '{"data":{"repository":{'
sep=""
for arg in "$@"; do
  case "$arg" in
    b[0-9]*=*)
      alias=${arg%%=*} name=${arg#*=}
      [ "$name" = "` + failing + `" ] && { echo "boom" >&2; exit 1; }
      printf '%s"%s":{"nodes":[{"number":1,"url":"%s","state":"MERGED","headRefName":"%s","isCrossRepository":false}]}' "$sep" "$alias" "$name" "$name"
      printf '%s ' "$name" >>` + log + `
      sep=",";;
  esac
done
echo >>` + log + `
printf '}}}'
`
	require.NoError(t, os.WriteFile(path, []byte(script), 0o755))
	return path, log
}

// branchesNamed are n branch names, b1 to bn.
func branchesNamed(n int) []string {
	names := make([]string, 0, n)
	for i := range n {
		names = append(names, "b"+strconv.Itoa(i+1))
	}
	return names
}
