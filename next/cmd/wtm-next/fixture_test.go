package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/adapter/events"
	"github.com/LucasPcq/wtm/next/internal/adapter/gittest"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// recorded publishes nothing and records what the publisher of each operation would stamp on its events.
type recorded struct {
	wireParams
	published *[]events.PublisherParams
}

func (r recorded) Open(op domain.Operation) domain.Report {
	*r.published = append(*r.published, r.publisherParams(op))
	return func(domain.Event) {}
}

// A mutation is a command of the catalogue the contract drives through api alone, on a repository configured for wtm: the answers that change it, its subject's first, whether its empty request is complete already (every field has a default), what it needs first, what makes its preview stale, what a refused apply leaves as it was, and what its apply leaves.
type mutation struct {
	name      string
	answers   []domain.Answered
	complete  bool
	setup     func(t *testing.T, main string)
	stale     func(t *testing.T, main string)
	untouched func(t *testing.T, main string)
	applied   func(t *testing.T, main string)
}

// mutations are the commands of the catalogue that change a repository.
func mutations() []mutation {
	return []mutation{
		{
			name:    "create",
			answers: []domain.Answered{{Path: "branches", Values: []string{"feat/x"}}},
			// the env strategy settles where the new worktree's .env comes from: a plan made before no longer holds
			stale: func(t *testing.T, main string) {
				writeConfig(t, main, "[env]\nstrategy = \"main\"\n")
			},
			untouched: func(t *testing.T, main string) {
				assert.NoDirExists(t, filepath.Join(filepath.Dir(main), ".trees", "feat-x"))
			},
			applied: func(t *testing.T, main string) {
				assert.DirExists(t, filepath.Join(filepath.Dir(main), ".trees", "feat-x"))
			},
		},
		{
			name:    "extract",
			answers: []domain.Answered{{Path: "source", Values: []string{"main"}}, {Path: "files", Values: []string{"notes.txt"}}, {Path: "to", Values: []string{"feat/notes"}}},
			setup: func(t *testing.T, main string) {
				require.NoError(t, os.WriteFile(filepath.Join(main, "notes.txt"), []byte("notes\n"), 0o644))
			},
			// staging the file changes its status, which the plan moves by
			stale: func(t *testing.T, main string) {
				gittest.Git(t, main, "add", "notes.txt")
			},
			untouched: func(t *testing.T, main string) {
				assert.FileExists(t, filepath.Join(main, "notes.txt"))
				assert.NoDirExists(t, filepath.Join(filepath.Dir(main), ".trees", "feat-notes"))
			},
			applied: func(t *testing.T, main string) {
				assert.FileExists(t, filepath.Join(filepath.Dir(main), ".trees", "feat-notes", "notes.txt"))
				assert.NoFileExists(t, filepath.Join(main, "notes.txt"))
			},
		},
		{
			name:    "clean",
			answers: []domain.Answered{{Path: domain.PathWorktree, Values: []string{"feat/old"}}},
			setup: func(t *testing.T, main string) {
				gittest.Git(t, main, "worktree", "add", "-q", "-b", "feat/old", filepath.Join(filepath.Dir(main), ".trees", "feat-old"))
			},
			// an on_clean hook is part of what the plan runs
			stale: func(t *testing.T, main string) {
				writeConfig(t, main, "[hooks]\non_clean = [\"true\"]\n")
			},
			untouched: func(t *testing.T, main string) {
				assert.DirExists(t, filepath.Join(filepath.Dir(main), ".trees", "feat-old"))
			},
			applied: func(t *testing.T, main string) {
				assert.NoDirExists(t, filepath.Join(filepath.Dir(main), ".trees", "feat-old"))
				assert.Empty(t, gittest.Output(t, main, "branch", "--list", "feat/old"))
			},
		},
		{
			name:     "prune",
			answers:  []domain.Answered{{Path: domain.PathWorktree, Values: []string{"feat/done"}}, {Path: "gone", Values: []string{"true"}}},
			complete: true,
			// origin deleted the branch feat/done tracks: done with, for the one reason no forge is needed for
			setup: func(t *testing.T, main string) {
				gittest.Git(t, main, "worktree", "add", "-q", "-b", "feat/done", filepath.Join(filepath.Dir(main), ".trees", "feat-done"))
				gittest.PushBranch(t, main, "feat/done")
				gittest.Git(t, gittest.Output(t, main, "remote", "get-url", "origin"), "update-ref", "-d", "refs/heads/feat/done")
			},
			// an on_clean hook is part of what the plan runs
			stale: func(t *testing.T, main string) {
				writeConfig(t, main, "[hooks]\non_clean = [\"true\"]\n")
			},
			untouched: func(t *testing.T, main string) {
				assert.DirExists(t, filepath.Join(filepath.Dir(main), ".trees", "feat-done"))
			},
			applied: func(t *testing.T, main string) {
				assert.NoDirExists(t, filepath.Join(filepath.Dir(main), ".trees", "feat-done"))
				assert.Empty(t, gittest.Output(t, main, "branch", "--list", "feat/done"))
			},
		},
		{
			name:     "fast-forward",
			answers:  []domain.Answered{{Path: domain.PathWorktree, Values: []string{"feat/ff"}}},
			complete: true,
			setup: func(t *testing.T, main string) {
				gittest.Git(t, main, "worktree", "add", "-q", "-b", "feat/ff", filepath.Join(filepath.Dir(main), ".trees", "feat-ff"))
				gittest.PushBranch(t, main, "feat/ff")
				pushOnOrigin(t, main, "feat/ff")
			},
			// origin moved on since: the plan's tip is no longer the one to move to
			stale: func(t *testing.T, main string) {
				pushOnOrigin(t, main, "feat/ff")
			},
			untouched: func(t *testing.T, main string) {
				assert.Equal(t, gittest.Output(t, main, "rev-parse", "main"), gittest.Output(t, main, "rev-parse", "feat/ff"))
			},
			applied: func(t *testing.T, main string) {
				assert.Equal(t, gittest.Output(t, main, "rev-parse", "origin/feat/ff"), gittest.Output(t, main, "rev-parse", "feat/ff"))
			},
		},
		{
			name:    "exec",
			answers: []domain.Answered{{Path: domain.PathWorktree, Values: []string{"feat/old"}}, {Path: "command", Values: []string{"touch ran"}}},
			setup: func(t *testing.T, main string) {
				gittest.Git(t, main, "worktree", "add", "-q", "-b", "feat/old", filepath.Join(filepath.Dir(main), ".trees", "feat-old"))
			},
			// the command runs from its worktree's root: moved, it would run elsewhere
			stale: func(t *testing.T, main string) {
				gittest.Git(t, main, "worktree", "move", filepath.Join(filepath.Dir(main), ".trees", "feat-old"), filepath.Join(filepath.Dir(main), ".trees", "feat-moved"))
			},
			untouched: func(t *testing.T, main string) {
				assert.NoFileExists(t, filepath.Join(filepath.Dir(main), ".trees", "feat-moved", "ran"))
			},
			applied: func(t *testing.T, main string) {
				assert.FileExists(t, filepath.Join(filepath.Dir(main), ".trees", "feat-moved", "ran"))
			},
		},
		{
			name:    "reparent",
			answers: []domain.Answered{{Path: domain.PathWorktree, Values: []string{"feat/b"}}, {Path: "to", Values: []string{"feat/a"}}},
			setup: func(t *testing.T, main string) {
				for _, branch := range []string{"feat/a", "feat/b"} {
					gittest.Git(t, main, "worktree", "add", "-q", "-b", branch, filepath.Join(filepath.Dir(main), ".trees", worktree.PathSegment(branch)))
					recordParent(t, main, branch, "main")
				}
			},
			// the parent it leaves is part of the plan
			stale: func(t *testing.T, main string) {
				recordParent(t, main, "feat/b", "topic")
			},
			untouched: func(t *testing.T, main string) {
				assert.Equal(t, "topic", parentOf(t, main, "feat/b"))
			},
			applied: func(t *testing.T, main string) {
				assert.Equal(t, "feat/a", parentOf(t, main, "feat/b"))
			},
		},
	}
}

// pushOnOrigin moves origin's branch name on by one commit the repository at main has not fetched, as a teammate's push does.
func pushOnOrigin(t *testing.T, main, name string) {
	t.Helper()
	parent := gittest.Output(t, main, "ls-remote", "origin", "refs/heads/"+name)[:40]
	gittest.Git(t, main, "fetch", "-q", "origin", name)
	commit := gittest.Output(t, main, "commit-tree", parent+"^{tree}", "-p", parent, "-m", "pushed")
	gittest.Git(t, main, "push", "-q", "origin", commit+":refs/heads/"+name)
	gittest.Git(t, main, "update-ref", "refs/remotes/origin/"+name, parent)
}

// reads are the commands of the catalogue that change nothing.
var reads = []string{"list", "resolve", "status", "tree"}

// streamsListed are the commands that stream rather than return: out of the catalogue's contract, each a stream of catalogue.go's streams.
var streamsListed = []string{"events"}

// firstLine keeps what a stream writes and stops it with stop once its first line is written.
type firstLine struct {
	bytes.Buffer
	stop func()
}

func (f *firstLine) Write(p []byte) (int, error) {
	n, err := f.Buffer.Write(p)
	if bytes.ContainsRune(p, '\n') {
		f.stop()
	}
	return n, err
}

// fieldKinds is one field of each kind a form holds.
func fieldKinds() []domain.Field {
	return []domain.Field{domain.SelectField{}, domain.MultiSelectField{}, domain.TextField{}, domain.TextListField{}, domain.ConfirmField{}}
}

// eventsOfEachType is one event of each type an operation reports or the event stream carries.
func eventsOfEachType() []domain.Event {
	identity := worktree.Identity{Branch: "feat/a", Path: "/repo/.trees/feat-a", Parent: "main", Isolation: "isolated", CreatedAt: "2026-10-24T12:00:00Z"}
	return []domain.Event{
		domain.UnitStarted{Subject: "feat/a"},
		domain.PhaseStarted{Subject: "feat/a", Phase: domain.PhaseOnCreate, Log: "/repo/.git/wtm/logs/feat-a.log"},
		domain.PhaseFinished{Subject: "feat/a", Phase: domain.PhaseOnCreate, Failed: true},
		domain.Output{Subject: "feat/a", Line: "installing"},
		domain.UnitFinished{Subject: "feat/a", Status: domain.StatusDone},
		domain.WorktreeCreated{Worktree: identity},
		domain.WorktreeUpdated{Worktree: identity, Changed: []worktree.IdentityField{worktree.IdentityOrdinal}},
		domain.WorktreeProvisioned{Worktree: identity, Hook: "make setup", ExitCode: new(2)},
		domain.WorktreeRemoved{Worktree: identity},
		domain.OperationStarted{Command: "create", Surface: domain.SurfaceAPI},
		domain.OperationFinished{Command: "create", Surface: domain.SurfaceAPI, Code: "unit.phase_failed"},
		domain.Snapshot{},
		domain.Ready{},
		domain.RepoAdded{},
		domain.RepoRemoved{},
	}
}
