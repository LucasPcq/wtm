package extract_test

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/extract"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestARepositoryWithoutConfigIsAPrecondition(t *testing.T) {
	w := newWorld(t, seed{})
	require.NoError(t, removeFile(filepath.Join(w.stateDir, "config.toml")))

	_, err := w.usecase.Observe(t.Context(), moving("a.txt"))

	failure := failureOf(t, err)
	assert.Equal(t, domain.KindPrecondition, failure.Kind)
	assert.Equal(t, extract.CodeConfigNotFound, failure.Code)
	assert.True(t, errors.Is(err, project.ErrConfigNotFound))
}

func TestThePlanIsTheJSONOfWhatMovesWhere(t *testing.T) {
	w := newWorld(t, seed{})
	w.change(t, source, "a.txt", "a\n")

	planned, _ := w.plan(t, extract.Request{Source: source, Files: []string{"a.txt"}, To: split})

	assert.JSONEq(t, `{
		"source": "feat/login",
		"files": [{"path": "a.txt", "status": "untracked"}],
		"target": {"branch": "feat/split", "path": "`+w.path(split)+`", "action": "create"},
		"from": "main",
		"keep": false,
		"conflicts": [],
		"on_conflict": "abort"
	}`, jsonOf(t, planned))
	assert.Equal(t, []domain.Lock{
		{Scope: domain.ScopeRepo, Mode: domain.LockShared},
		{Scope: domain.ScopeWorktree, Name: source, Mode: domain.LockExclusive},
		{Scope: domain.ScopeWorktree, Name: split, Mode: domain.LockExclusive},
	}, planned.Locks())
}

func TestARequestLocksItsSourceAndTarget(t *testing.T) {
	assert.Equal(t, []domain.Lock{
		{Scope: domain.ScopeRepo, Mode: domain.LockShared},
		{Scope: domain.ScopeWorktree, Name: source, Mode: domain.LockExclusive},
	}, extract.Request{Source: source}.Locks())
}

func TestABranchCheckedOutSomewhereIsAnExistingTarget(t *testing.T) {
	w := newWorld(t, seed{})
	w.change(t, source, "a.txt", "a\n")

	planned, _ := w.plan(t, moving("a.txt"))

	assert.Equal(t, extract.Target{Branch: other, Path: w.at(other), Action: extract.ActionExisting}, planned.Target)
	assert.Empty(t, planned.From)
}

func TestEachBranchStateGivesItsAction(t *testing.T) {
	w := newWorld(t, seed{})
	w.change(t, source, "a.txt", "a\n")

	checkedOut, _ := w.plan(t, extract.Request{Source: source, Files: []string{"a.txt"}, To: existing, From: "main"})
	resumed, _ := w.plan(t, extract.Request{Source: source, Files: []string{"a.txt"}, To: remote})

	assert.Equal(t, extract.ActionCheckOut, checkedOut.Target.Action)
	assert.Equal(t, extract.Target{Branch: remote, Path: w.path(remote), Action: extract.ActionResume, Upstream: "origin/" + remote}, resumed.Target)
}

func TestConflictsUnderResolveArePlanned(t *testing.T) {
	w := newWorld(t, seed{})
	w.change(t, source, "a.txt", "a\n")
	conflicts := []worktree.Conflict{{Path: "a.txt", Kind: worktree.ConflictExists}}

	planned, _, err := extract.Decide(w.facts(t, moving("a.txt")), extract.Request{Source: source, Files: []string{"a.txt"}, To: other, OnConflict: extract.OnConflictResolve}, conflicts)

	require.NoError(t, err)
	assert.Equal(t, conflicts, planned.Conflicts)
}

func TestABinaryCollisionIsRefusedEvenUnderResolve(t *testing.T) {
	w := newWorld(t, seed{})
	w.change(t, source, "a.txt", "a\n")

	_, _, err := extract.Decide(w.facts(t, moving("a.txt")), extract.Request{Source: source, Files: []string{"a.txt"}, To: other, OnConflict: extract.OnConflictResolve}, []worktree.Conflict{
		{Path: "a.go", Kind: worktree.ConflictModified},
		{Path: "logo.png", Kind: worktree.ConflictBinary},
	})

	failure := failureOf(t, err)
	assert.Equal(t, extract.CodeConflict, failure.Code)
	assert.Equal(t, `cannot apply the selected changes: logo.png already exists in "feat/other" and is binary — conflict markers cannot be written, remove it there or extract to another worktree`, failure.Message)
	assert.Equal(t, domain.Params{"target": other, "files": "logo.png"}, failure.Params)
}

func TestAConflictRefusalNamesWhatWasModifiedAndWhatAlreadyExists(t *testing.T) {
	w := newWorld(t, seed{})
	w.change(t, source, "a.txt", "a\n")

	_, _, err := extract.Decide(w.facts(t, moving("a.txt")), extract.Request{Source: source, Files: []string{"a.txt"}, To: other, OnConflict: extract.OnConflictAbort}, []worktree.Conflict{
		{Path: "a.go", Kind: worktree.ConflictModified},
		{Path: "b.go", Kind: worktree.ConflictModified},
		{Path: "new.txt", Kind: worktree.ConflictExists},
	})

	failure := failureOf(t, err)
	assert.Equal(t, domain.KindConflict, failure.Kind)
	assert.Equal(t, `cannot apply the selected changes: a.go, b.go were also modified and new.txt already exists in "feat/other" — resolve them there, extract to another worktree, or re-run with --on-conflict resolve`, failure.Message)
}

func TestAnExistingTargetIgnoresTheCreationAnswersAndSaysSo(t *testing.T) {
	w := newWorld(t, seed{})
	w.change(t, source, "a.txt", "a\n")
	req := moving("a.txt")
	req.From, req.FastForward, req.Isolation = "main", new(true), project.Verbatim

	planned, warnings := w.plan(t, req)

	assert.Equal(t, []string{"extract.from_ignored", "extract.fast_forward_ignored", "extract.isolation_ignored"}, codes(warnings))
	assert.Equal(t, "--isolation verbatim ignored: feat/other already exists and stays isolated — switch it with `wtm env feat/other --isolation verbatim`", warnings[2].Message)
	assert.Empty(t, planned.From)
	assert.Empty(t, planned.FastForward)
	assert.Empty(t, planned.Isolation)
}

func TestAnIsolationTheTargetAlreadyHasIsNotWarnedAbout(t *testing.T) {
	w := newWorld(t, seed{})
	w.change(t, source, "a.txt", "a\n")
	w.record(t, other, worktree.Meta{SourceBranch: "main", Isolation: project.Verbatim})
	req := moving("a.txt")
	req.Isolation = project.Verbatim

	_, warnings := w.plan(t, req)

	assert.Empty(t, warnings)
}
