package github_test

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

// fakeGH writes a gh that answers auth status with authExit, saying what gh says when signed out, and any other call with answer, and returns its path.
func fakeGH(t *testing.T, authExit int, answer string) string {
	t.Helper()
	return authGH(t, authExit, "You are not logged into any GitHub hosts. To log in, run: gh auth login", answer)
}

// authGH writes a gh whose auth status prints said on its stderr and exits with authExit, and which answers any other call with answer.
func authGH(t *testing.T, authExit int, said, answer string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "gh")
	script := "#!/bin/sh\nif [ \"$1\" = auth ]; then echo '" + said + "' >&2; exit " + strconv.Itoa(authExit) + "; fi\ncat <<'JSON'\n" + answer + "\nJSON\n"
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

// loggedGH writes a gh that answers auth status and any other call with answer, appending each call's arguments to the log it returns.
func loggedGH(t *testing.T, answer string) (string, string) {
	t.Helper()
	dir := t.TempDir()
	path, log := filepath.Join(dir, "gh"), filepath.Join(dir, "calls.log")
	script := "#!/bin/sh\nif [ \"$1\" = auth ]; then exit 0; fi\necho \"$*\" >>" + log + "\ncat <<'JSON'\n" + answer + "\nJSON\n"
	require.NoError(t, os.WriteFile(path, []byte(script), 0o755))
	return path, log
}

// refusingGH writes a gh that answers auth status and fails any other call with message on its stderr.
func refusingGH(t *testing.T, message string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "gh")
	script := "#!/bin/sh\nif [ \"$1\" = auth ]; then exit 0; fi\necho '" + message + "' >&2\nexit 1\n"
	require.NoError(t, os.WriteFile(path, []byte(script), 0o755))
	return path
}

func readLog(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(content)
}
