// Package ghtest drives the GitHub CLI a test sees. `service/github` shells out
// to `gh`, so a script first on PATH is enough to script PR state without a
// network, and dropping every PATH entry that holds one is enough to simulate
// its absence.
package ghtest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// PR is one pull request the stubbed `gh` reports.
type PR struct {
	Number int
	Branch string
	State  string
}

type StubParams struct {
	PRs []PR
	// Unauthenticated makes `gh auth status` fail, which the service maps to
	// domain.GHConnectionNotAuthenticated.
	Unauthenticated bool
	// Details answer `gh pr view <number>`; a number missing from them fails as
	// gh does for a pull request that does not exist.
	Details []PRDetail
}

// PRDetail is one pull request as `gh pr view --json` describes it.
type PRDetail struct {
	Number int
	Title  string
	Author string
	Branch string
	Base   string
	Fork   bool
	Draft  bool
}

type ghAuthor struct {
	Login string `json:"login"`
}

type ghPRDetail struct {
	Number            int      `json:"number"`
	Title             string   `json:"title"`
	Author            ghAuthor `json:"author"`
	HeadRefName       string   `json:"headRefName"`
	BaseRefName       string   `json:"baseRefName"`
	URL               string   `json:"url"`
	IsCrossRepository bool     `json:"isCrossRepository"`
	IsDraft           bool     `json:"isDraft"`
}

// ghPR is the shape service/github parses out of `gh pr list --json`.
type ghPR struct {
	Number      int    `json:"number"`
	HeadRefName string `json:"headRefName"`
	URL         string `json:"url"`
	State       string `json:"state"`
}

// Stub puts a fake `gh` first on PATH for the rest of the test. Like the real
// CLI, `pr list` returns at most --limit pull requests (newest first: the order
// of PRs), and `api graphql` answers each branch it is asked about with the
// first pull request of that branch.
func Stub(t testing.TB, params StubParams) {
	t.Helper()

	dir := t.TempDir()
	listing := filepath.Join(dir, "prs.jsonl")
	if err := os.WriteFile(listing, []byte(prLines(t, params.PRs)), 0o644); err != nil {
		t.Fatalf("write stub PRs: %v", err)
	}

	authExit := 0
	if params.Unauthenticated {
		authExit = 1
	}

	script := fmt.Sprintf(`#!/bin/sh
listing=%q
case "$1" in
  auth) exit %d ;;
  api)
    printf '['
    sep="" prev=""
    for arg in "$@"; do
      if [ "$prev" = "-f" ]; then
        case "$arg" in
          b[0-9]*=*)
            branch="${arg#*=}"
            pr=$(awk -v b="$branch" -F '\t' '$1 == b { print $2; exit }' "$listing")
            if [ -n "$pr" ]; then printf '%%s%%s' "$sep" "$pr"; sep=","; fi ;;
        esac
      fi
      prev="$arg"
    done
    printf ']\n'
    exit 0 ;;
  pr)
    if [ "$2" = "view" ]; then
      case "$3" in
%s      *) echo "no pull requests found for number $3" >&2; exit 1 ;;
      esac
    fi
    limit=30 prev=""
    for arg in "$@"; do
      if [ "$prev" = "--limit" ]; then limit="$arg"; fi
      prev="$arg"
    done
    printf '['
    head -n "$limit" "$listing" | cut -f 2 | paste -sd ',' -
    printf ']\n'
    exit 0 ;;
esac
exit 1
`, listing, authExit, detailCases(t, params.Details))

	if err := os.WriteFile(filepath.Join(dir, "gh"), []byte(script), 0o755); err != nil {
		t.Fatalf("write gh stub: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// prLines writes one "<branch>\t<json>" line per pull request, in order.
func prLines(t testing.TB, prs []PR) string {
	t.Helper()
	var lines strings.Builder
	for _, pr := range prs {
		payload, err := json.Marshal(ghPR{
			Number:      pr.Number,
			HeadRefName: pr.Branch,
			URL:         fmt.Sprintf("https://github.com/test/test/pull/%d", pr.Number),
			State:       strings.ToUpper(pr.State),
		})
		if err != nil {
			t.Fatalf("marshal stub PR: %v", err)
		}
		fmt.Fprintf(&lines, "%s\t%s\n", pr.Branch, payload)
	}
	return lines.String()
}

func detailCases(t testing.TB, details []PRDetail) string {
	t.Helper()
	var cases strings.Builder
	for _, detail := range details {
		payload, err := json.Marshal(ghPRDetail{
			Number:            detail.Number,
			Title:             detail.Title,
			Author:            ghAuthor{Login: detail.Author},
			HeadRefName:       detail.Branch,
			BaseRefName:       detail.Base,
			URL:               fmt.Sprintf("https://github.com/test/test/pull/%d", detail.Number),
			IsCrossRepository: detail.Fork,
			IsDraft:           detail.Draft,
		})
		if err != nil {
			t.Fatalf("marshal stub PR detail: %v", err)
		}
		fmt.Fprintf(&cases, "        %d) cat <<'WTM_GH_STUB_EOF'\n%s\nWTM_GH_STUB_EOF\n          exit 0 ;;\n", detail.Number, payload)
	}
	return cases.String()
}

// Absent hides `gh` from exec.LookPath. A PATH entry that holds a `gh` is not
// dropped but replaced, in place and in order, by a mirror of itself without it:
// on a CI runner `gh` and `git` share /usr/bin, so dropping the entry would take
// git down with it.
func Absent(t testing.TB) {
	t.Helper()

	entries := filepath.SplitList(os.Getenv("PATH"))
	sanitized := make([]string, 0, len(entries))
	for _, dir := range entries {
		if dir == "" {
			continue
		}
		if !holdsGH(dir) {
			sanitized = append(sanitized, dir)
			continue
		}
		sanitized = append(sanitized, mirrorWithoutGH(t, dir))
	}
	t.Setenv("PATH", strings.Join(sanitized, string(os.PathListSeparator)))
}

func holdsGH(dir string) bool {
	info, err := os.Stat(filepath.Join(dir, "gh"))
	return err == nil && !info.IsDir()
}

// mirrorWithoutGH symlinks everything a directory holds except `gh`, so the rest
// of it keeps resolving at the same position in PATH.
func mirrorWithoutGH(t testing.TB, dir string) string {
	t.Helper()

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	mirror := t.TempDir()
	for _, entry := range entries {
		if entry.Name() == "gh" {
			continue
		}
		// A name that cannot be mirrored is one the test may need, but it is not
		// worth failing over: PATH lookup simply falls through to a later entry.
		_ = os.Symlink(filepath.Join(dir, entry.Name()), filepath.Join(mirror, entry.Name()))
	}
	return mirror
}
