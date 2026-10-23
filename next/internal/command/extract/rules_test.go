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
		"env_from": "example",
		"creates": 1,
		"keep": false,
		"conflicts": [],
		"on_conflict": "abort",
		"recap": [
			{"key": "source", "label": "Source", "path": "source", "values": [{"value": "feat/login", "text": "feat/login"}]},
			{"key": "files", "label": "Files", "path": "files", "values": [{"value": "a.txt", "note": "untracked", "text": "new a.txt"}]},
			{"key": "target", "label": "Target", "path": "to", "values": [{"value": "feat/split", "note": "new", "params": {"from": "main"}, "text": "new worktree feat/split from main"}]},
			{"key": "env_from", "label": "Env", "path": "env_from", "values": [{"value": "example", "text": "example"}]},
			{"key": "mode", "label": "Mode", "path": "keep", "values": [{"value": "move", "text": "move"}]}
		]
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

func TestTheRecapSpellsHowTheTargetGetsItsWorktree(t *testing.T) {
	w := newWorld(t, seed{})
	w.change(t, source, "a.txt", "a\n")

	checkedOut, _ := w.plan(t, extract.Request{Source: source, Files: []string{"a.txt"}, To: existing, From: "main"})
	resumed, _ := w.plan(t, extract.Request{Source: source, Files: []string{"a.txt"}, To: remote})

	assert.Equal(t, domain.RecapValue{Value: existing, Note: worktree.NoteExisting, Text: existing + " (existing local branch — reused)"}, checkedOut.Recap[2].Values[0])
	assert.Equal(t, domain.RecapRow{Key: "parent", Label: "Parent", Path: "from", Values: domain.RecapValues([]string{"main"})}, checkedOut.Recap[3])
	assert.Equal(t, domain.RecapValue{Value: remote, Note: worktree.NoteResumes, Params: domain.Params{"upstream": "origin/" + remote}, Text: remote + " (resumes origin/" + remote + ")"}, resumed.Recap[2].Values[0])
	assert.NotEqual(t, "parent", resumed.Recap[3].Key)
}

func TestTheRecapAndTheRecalledAnswersLeaveTheFingerprintAlone(t *testing.T) {
	w := newWorld(t, seed{Config: remembering, Run: jobsToml})
	w.change(t, source, "a.txt", "a\n")
	planned, _ := w.plan(t, extract.Request{Source: source, Files: []string{"a.txt"}, To: split})
	bare := planned
	bare.Recalled, bare.Recap = nil, nil

	assert.NotEmpty(t, planned.Recalled)
	assert.Equal(t, jsonOf(t, bare.Fingerprinted()), jsonOf(t, planned.Fingerprinted()))
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
	req.From, req.EnvFrom, req.FastForward, req.Isolation = "main", project.EnvMain, new(true), project.Verbatim

	planned, warnings := w.plan(t, req)

	assert.Equal(t, []string{"extract.from_ignored", "extract.env_from_ignored", "extract.fast_forward_ignored", "extract.isolation_ignored"}, codes(warnings))
	assert.Equal(t, "env strategy ignored: feat/other already exists, so nothing is created", warnings[1].Message)
	assert.Equal(t, "fast-forward ignored: feat/other already exists, so nothing is created", warnings[2].Message)
	assert.Equal(t, "isolation verbatim ignored: feat/other already exists and stays isolated", warnings[3].Message)
	assert.Equal(t, &domain.FollowUp{
		Command: domain.CommandEnv,
		Answers: []domain.Answered{{Path: domain.PathWorktree, Values: []string{"feat/other"}}, {Path: domain.PathIsolation, Values: []string{"verbatim"}}},
		Code:    extract.CodeSwitchIsolation,
		Message: "switch it to verbatim",
	}, warnings[3].FollowUp)
	assert.Empty(t, planned.From)
	assert.Empty(t, planned.EnvFrom)
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

func TestARequestedPathSelectsTheChangeItNamesFirst(t *testing.T) {
	available := []worktree.Change{{Path: "api"}, {Path: "api/a.go"}, {Path: "web/b.go"}}

	selected, unmatched := extract.SelectChanges(available, []string{"api"})

	assert.Equal(t, []worktree.Change{{Path: "api"}}, selected)
	assert.Empty(t, unmatched)
}

func TestARequestedDirectorySelectsEveryChangeUnderItWithOrWithoutItsSlash(t *testing.T) {
	available := []worktree.Change{{Path: "api/a.go"}, {Path: "apix.go"}, {Path: "api/new/b.go"}, {Path: "web/c.go"}}

	bare, _ := extract.SelectChanges(available, []string{"api"})
	slashed, _ := extract.SelectChanges(available, []string{"api/"})

	assert.Equal(t, []worktree.Change{{Path: "api/a.go"}, {Path: "api/new/b.go"}}, bare)
	assert.Equal(t, bare, slashed)
}

func TestASelectionKeepsEachChangeOnceInTheOrderItWasAskedFor(t *testing.T) {
	available := []worktree.Change{{Path: "api/a.go"}, {Path: "web/c.go"}, {Path: "api/b.go"}}

	selected, unmatched := extract.SelectChanges(available, []string{"web/c.go", "api/b.go", "api"})

	assert.Equal(t, []worktree.Change{{Path: "web/c.go"}, {Path: "api/b.go"}, {Path: "api/a.go"}}, selected)
	assert.Empty(t, unmatched)
}

func TestASelectionNamesThePathThatMatchedNothing(t *testing.T) {
	available := []worktree.Change{{Path: "DOC.md", OrigPath: "README.md", Status: worktree.ChangeRenamed}}

	selected, unmatched := extract.SelectChanges(available, []string{"DOC.md", "README.md"})

	assert.Nil(t, selected)
	assert.Equal(t, "README.md", unmatched)
}
