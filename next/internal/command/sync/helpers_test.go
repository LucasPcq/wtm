package sync_test

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/adapter/gittest"
	"github.com/LucasPcq/wtm/next/internal/command/sync"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/stretchr/testify/require"
)

func (w world) at(name string) string {
	if name == "main" {
		return w.main
	}
	return linkedPath(w.root, name)
}

func (w world) record(t *testing.T, branch string, meta worktree.Meta) {
	t.Helper()
	_, err := w.meta.Update(t.Context(), worktree.MetaUpdate{Branch: branch, Change: func(stored *worktree.Meta) error {
		*stored = meta
		return nil
	}})
	require.NoError(t, err)
}

// recordParent rewrites the parent the record of branch names, keeping the rest.
func (w world) recordParent(t *testing.T, branch, parent string) {
	t.Helper()
	_, err := w.meta.Update(t.Context(), worktree.MetaUpdate{Branch: branch, Change: func(stored *worktree.Meta) error {
		stored.SourceBranch = parent
		return nil
	}})
	require.NoError(t, err)
}

// commit commits file with content in the worktree of branch.
func (w world) commit(t *testing.T, branch, file, content string) {
	t.Helper()
	writeFile(t, filepath.Join(w.at(branch), file), content+"\n")
	gittest.Git(t, w.at(branch), "add", file)
	gittest.Git(t, w.at(branch), "commit", "-q", "-m", file+": "+content)
}

// startFrom moves the branch of the worktree of branch to where from stands, as if it was created from it now.
func (w world) startFrom(t *testing.T, branch, from string) {
	t.Helper()
	gittest.Git(t, w.at(branch), "reset", "-q", "--hard", from)
}

// conflicting makes main and feat/b change shared.txt each their own way, so feat/b's rebase onto feat/a, once feat/a holds main's, stops on a conflict.
func (w world) conflicting(t *testing.T) {
	t.Helper()
	w.commit(t, featB, "shared.txt", "b's line")
	w.startFrom(t, featC, featB)
	w.commit(t, featC, "c2.txt", "more c")
	w.commit(t, "main", "shared.txt", "main's line")
}

// resolve finishes the rebase stopped in the worktree of branch as a user does: a line of its own, added, then continued.
func (w world) resolve(t *testing.T, branch string) {
	t.Helper()
	writeFile(t, filepath.Join(w.at(branch), "shared.txt"), "resolved\n")
	gittest.Git(t, w.at(branch), "add", "shared.txt")
	gittest.Git(t, w.at(branch), "-c", "core.editor=true", "rebase", "--continue")
}

func (w world) tip(t *testing.T, ref string) string {
	t.Helper()
	return gittest.Output(t, w.main, "rev-parse", ref)
}

// holds reports whether the branch holds the commit ref stands at.
func (w world) holds(t *testing.T, branch, ref string) bool {
	t.Helper()
	return gittest.Output(t, w.main, "merge-base", branch, ref) == w.tip(t, ref)
}

func (w world) run(t *testing.T, req sync.Request) sync.Outcome {
	t.Helper()
	outcome, err := w.usecase.Run(t.Context(), req, domain.RunOptions{Surface: "cli"})
	require.NoError(t, err)
	return outcome
}

func (w world) plan(t *testing.T, req sync.Request) (sync.Plan, []domain.Warning) {
	t.Helper()
	planned, warnings, err := w.usecase.Plan(t.Context(), req, nil)
	require.NoError(t, err)
	return planned, warnings
}

// pushedOnOrigin moves origin's branch name on by one commit, as a teammate's push does.
func (w world) pushedOnOrigin(t *testing.T, name string) {
	t.Helper()
	commit := gittest.Output(t, w.main, "commit-tree", "origin/"+name+"^{tree}", "-p", "origin/"+name, "-m", "pushed since")
	gittest.Git(t, w.main, "push", "-q", "origin", commit+":refs/heads/"+name)
}

func naming(names ...string) sync.Request {
	return sync.Request{Worktrees: names}
}

func failureOf(t *testing.T, err error) *domain.Error {
	t.Helper()
	failure, ok := errors.AsType[*domain.Error](err)
	require.True(t, ok, err)
	return failure
}

func itemsOf(outcome sync.Outcome) map[string]sync.Item {
	found := map[string]sync.Item{}
	for _, item := range outcome.Result.Items {
		found[item.Subject] = item
	}
	return found
}

func subjectsOf(outcome sync.Outcome) []string {
	var found []string
	for _, item := range outcome.Result.Items {
		found = append(found, item.Subject)
	}
	return found
}

func codes(warnings []domain.Warning) []string {
	found := make([]string, 0, len(warnings))
	for _, warning := range warnings {
		found = append(found, warning.Code)
	}
	return found
}

func followUpCodes(followUps []domain.FollowUp) []string {
	found := make([]string, 0, len(followUps))
	for _, followUp := range followUps {
		found = append(found, followUp.Code)
	}
	return found
}

func standings(planned sync.Plan) map[string]sync.Standing {
	found := map[string]sync.Standing{}
	for _, r := range planned.Worktrees {
		found[r.Worktree] = r.Standing
	}
	return found
}

func rebaseOf(planned sync.Plan, name string) sync.Rebase {
	return planned.Worktrees[slices.IndexFunc(planned.Worktrees, func(r sync.Rebase) bool { return r.Worktree == name })]
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}
