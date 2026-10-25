package resolve_test

import (
	"context"
	"errors"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/adapter/gittest"
	"github.com/LucasPcq/wtm/next/internal/command/resolve"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAWorktreeIsNamedByItsBranchItsPathOrAPartOfItsBranch(t *testing.T) {
	w := newWorld(seed{Git: gittest.FakeParams{Worktrees: several}})

	cases := map[string]struct {
		named, cwd string
		want       resolve.Detail
	}{
		"branch":        {named: "feat/login", want: resolve.Detail{Branch: "feat/login", Path: "/repo/.trees/feat/login"}},
		"main":          {named: "main", want: resolve.Detail{Branch: "main", Path: mainPath}},
		"part":          {named: "nav", want: resolve.Detail{Branch: "fix/nav", Path: "/repo/.trees/fix/nav"}},
		"path":          {named: "/repo/.trees/feat/logout", want: resolve.Detail{Branch: "feat/logout", Path: "/repo/.trees/feat/logout"}},
		"relative_path": {named: "../.trees/fix/nav", cwd: mainPath, want: resolve.Detail{Branch: "fix/nav", Path: "/repo/.trees/fix/nav"}},
		"detached":      {named: "/repo/.trees/detached", want: resolve.Detail{Path: "/repo/.trees/detached"}},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, c.want, resolved(t, w, resolve.Request{Worktree: c.named, Caller: domain.Caller{Cwd: c.cwd}}))
		})
	}
}

func TestAPartOfABranchWinsOverADirectoryOfTheSameName(t *testing.T) {
	w := newWorld(seed{Git: gittest.FakeParams{Worktrees: []worktree.Worktree{linked("feat/docs-site")}, Empty: []string{mainPath + "/docs"}}})

	got := resolved(t, w, resolve.Request{Worktree: "docs", Caller: domain.Caller{Cwd: mainPath}})

	assert.Equal(t, "feat/docs-site", got.Branch, "standing in main beside its docs/, wtm go docs enters feat/docs-site")
}

func TestWithNoWorktreeNamedTheOneHoldingTheDirectoryIsResolvedElseTheMainOne(t *testing.T) {
	w := newWorld(seed{Git: gittest.FakeParams{Worktrees: several}})

	cases := map[string]struct{ cwd, path string }{
		"inside_a_worktree": {cwd: "/repo/.trees/fix/nav/src", path: "/repo/.trees/fix/nav"},
		"detached":          {cwd: "/repo/.trees/detached", path: "/repo/.trees/detached"},
		"outside":           {cwd: "/elsewhere", path: mainPath},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, c.path, resolved(t, w, resolve.Request{Caller: domain.Caller{Cwd: c.cwd}}).Path)
		})
	}
}

func TestANameMatchingNoWorktreeOrSeveralIsRefused(t *testing.T) {
	w := newWorld(seed{Git: gittest.FakeParams{Worktrees: several}})

	cases := map[string]struct {
		named   string
		code    string
		matches string
	}{
		"not_found": {named: "nope", code: domain.CodeNotFound},
		"ambiguous": {named: "log", code: domain.CodeAmbiguous, matches: "feat/login, feat/logout"},
		"two_words": {named: "feat login", code: domain.CodeNotFound},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := w.usecase.Run(t.Context(), resolve.Request{Worktree: c.named}, domain.RunOptions{})

			failure, ok := errors.AsType[*domain.Error](err)
			require.True(t, ok, err)
			assert.Equal(t, domain.KindInvalid, failure.Kind)
			require.Len(t, failure.Fields, 1)
			assert.Equal(t, domain.PathWorktree, failure.Fields[0].Path)
			assert.Equal(t, c.code, failure.Fields[0].Code)
			assert.Equal(t, c.matches, failure.Fields[0].Params["matches"])
		})
	}
}

func TestAWorktreeWhoseDirectoryIsGoneIsResolvedMissing(t *testing.T) {
	w := newWorld(seed{Git: gittest.FakeParams{Worktrees: several, Gone: []string{"/repo/.trees/fix/nav"}}})

	assert.Equal(t, resolve.Detail{Branch: "fix/nav", Path: "/repo/.trees/fix/nav", Missing: true}, resolved(t, w, resolve.Request{Worktree: "fix/nav"}))
}

func TestAWorktreeGitCannotTellOfIsUnread(t *testing.T) {
	w := newWorld(seed{Git: gittest.FakeParams{Fail: map[string]error{"Locate": errors.New("git broke")}}})

	got, err := w.usecase.Run(t.Context(), resolve.Request{}, domain.RunOptions{})

	require.NoError(t, err)
	require.Len(t, got.Items, 1)
	assert.Equal(t, domain.StatusFailed, got.Items[0].Status)
	assert.Equal(t, resolve.CodeUnreadable, got.Failure().Code)
}

func TestAnInterruptedReadEndsCancelled(t *testing.T) {
	w := newWorld(seed{})
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	got, err := w.usecase.Run(ctx, resolve.Request{}, domain.RunOptions{})

	require.NoError(t, err)
	assert.Equal(t, domain.KindCancelled, got.Failure().Kind)
}

func TestAReadChangesNothing(t *testing.T) {
	w := newWorld(seed{Git: gittest.FakeParams{Worktrees: several}})

	resolved(t, w, resolve.Request{Worktree: "login"})

	for _, call := range w.git.Calls() {
		assert.Regexp(t, `^(Worktrees|Branches|AheadBehind|Locate) `, call)
	}
}

func TestARepositoryWithNoWtmConfigIsRefusedWithInitAsItsFollowUp(t *testing.T) {
	w := newWorld(seed{ConfigErr: project.ErrConfigNotFound})

	_, err := w.usecase.Run(t.Context(), resolve.Request{}, domain.RunOptions{})

	failure, ok := errors.AsType[*domain.Error](err)
	require.True(t, ok, err)
	assert.Equal(t, resolve.CodeConfigNotFound, failure.Code)
	require.NotNil(t, failure.FollowUp)
	assert.Equal(t, domain.CommandInit, failure.FollowUp.Command)
}
