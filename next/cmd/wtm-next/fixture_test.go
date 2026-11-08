package main

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/adapter/daemon"
	"github.com/LucasPcq/wtm/next/internal/adapter/events"
	"github.com/LucasPcq/wtm/next/internal/adapter/gittest"
	"github.com/LucasPcq/wtm/next/internal/command/daemonrestart"
	"github.com/LucasPcq/wtm/next/internal/command/daemonstop"
	"github.com/LucasPcq/wtm/next/internal/command/daemonstopjobs"
	"github.com/LucasPcq/wtm/next/internal/command/rundown"
	"github.com/LucasPcq/wtm/next/internal/command/runstart"
	"github.com/LucasPcq/wtm/next/internal/command/runstop"
	"github.com/LucasPcq/wtm/next/internal/command/runup"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
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
			name:    "checkout",
			answers: []domain.Answered{{Path: "pr", Values: []string{"7"}}},
			// pull request 7's branch is on origin alone, and gh is a stand-in that knows that pull request
			setup: func(t *testing.T, main string) {
				gittest.Git(t, main, "branch", "feat/pr")
				gittest.PushBranch(t, main, "feat/pr")
				gittest.Git(t, main, "branch", "-D", "feat/pr")
				fakeGH(t)
			},
			// the env strategy settles where the new worktree's .env comes from: a plan made before no longer holds
			stale: func(t *testing.T, main string) {
				writeConfig(t, main, "[env]\nstrategy = \"main\"\n")
			},
			untouched: func(t *testing.T, main string) {
				assert.NoDirExists(t, filepath.Join(filepath.Dir(main), ".trees", "feat-pr"))
			},
			applied: func(t *testing.T, main string) {
				assert.DirExists(t, filepath.Join(filepath.Dir(main), ".trees", "feat-pr"))
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
			name:     runup.Name,
			answers:  []domain.Answered{{Path: domain.PathWorktree, Values: []string{"main"}}},
			complete: true,
			// a task, which runs to its end, under a daemon of the test's own at the socket the commands reach, in a home short enough for it
			setup: func(t *testing.T, main string) {
				writeRun(t, main, seedJob)
				t.Setenv("HOME", shortDir(t))
				serving(t, daemonParams{Socket: daemon.SocketPath()})
			},
			// a job more in run.toml: the profile starts another
			stale: func(t *testing.T, main string) {
				writeRun(t, main, seedJob+"\n[[job]]\nname = \"lint\"\nkind = \"task\"\ncmd = \"true\"\n")
			},
			untouched: func(t *testing.T, _ string) {
				assert.Empty(t, daemonJobs(t))
			},
			applied: func(t *testing.T, _ string) {
				assert.Len(t, daemonJobs(t), 2)
			},
		},
		{
			name:     rundown.Name,
			answers:  []domain.Answered{{Path: domain.PathWorktree, Values: []string{"main"}}},
			complete: true,
			// a service up in main, under a daemon of the test's own at the socket the commands reach, in a home short enough for it
			setup: func(t *testing.T, main string) {
				writeRun(t, main, idleJob("idle"))
				t.Setenv("HOME", shortDir(t))
				serving(t, daemonParams{Socket: daemon.SocketPath()})
				startIdle(t, main, "idle")
			},
			// another job up: the plan stops one more
			stale: func(t *testing.T, main string) {
				startIdle(t, main, "other")
			},
			untouched: func(t *testing.T, _ string) {
				assert.True(t, slices.ContainsFunc(daemonJobs(t), func(job run.Job) bool { return job.Up }))
			},
			applied: func(t *testing.T, _ string) {
				assert.False(t, slices.ContainsFunc(daemonJobs(t), func(job run.Job) bool { return job.Up }))
			},
		},
		{
			name:     daemonstop.Name,
			complete: true,
			// a service up, under a daemon of the test's own at the socket the commands reach, in a home short enough for it
			setup: func(t *testing.T, main string) {
				writeRun(t, main, idleJob("idle"))
				t.Setenv("HOME", shortDir(t))
				serving(t, daemonParams{Socket: daemon.SocketPath()})
				startIdle(t, main, "idle")
			},
			// another service up: the plan stops one more
			stale: func(t *testing.T, main string) {
				startIdle(t, main, "other")
			},
			untouched: func(t *testing.T, _ string) {
				assert.True(t, slices.ContainsFunc(daemonJobs(t), func(job run.Job) bool { return job.Up }))
			},
			applied: func(t *testing.T, _ string) {
				status, err := wtmDaemon().Status(t.Context())
				require.NoError(t, err)
				assert.False(t, status.Answering, "the daemon still answers")
			},
		},
		{
			name:     daemonrestart.Name,
			complete: true,
			// a service up, under a daemon of the test's own at the socket the commands reach, which the restart replaces by another of its own
			setup: func(t *testing.T, main string) {
				writeRun(t, main, idleJob("idle"))
				t.Setenv("HOME", shortDir(t))
				serving(t, daemonParams{Socket: daemon.SocketPath()})
				restartable(t)
				startIdle(t, main, "idle")
			},
			// another service up: one more comes back
			stale: func(t *testing.T, main string) {
				startIdle(t, main, "other")
			},
			untouched: func(t *testing.T, _ string) {
				assert.Len(t, slices.DeleteFunc(daemonJobs(t), func(job run.Job) bool { return !job.Up }), 2)
			},
			// N4: the services are back on the new daemon
			applied: func(t *testing.T, _ string) {
				var names []string
				for _, job := range daemonJobs(t) {
					if job.Up {
						names = append(names, job.Name)
					}
				}
				assert.ElementsMatch(t, []string{"idle", "other"}, names)
			},
		},
		{
			name:     daemonstopjobs.Name,
			answers:  []domain.Answered{{Path: domain.PathRepo, Values: []string{"."}}},
			complete: true,
			// a service up in main, started for its repository, under a daemon of the test's own at the socket the commands reach, in a home short enough for it
			setup: func(t *testing.T, main string) {
				writeRun(t, main, idleJob("idle"))
				t.Setenv("HOME", shortDir(t))
				serving(t, daemonParams{Socket: daemon.SocketPath()})
				startIdleFor(t, main, "idle")
			},
			// another job up: the plan stops one more
			stale: func(t *testing.T, main string) {
				startIdleFor(t, main, "other")
			},
			untouched: func(t *testing.T, _ string) {
				assert.True(t, slices.ContainsFunc(daemonJobs(t), func(job run.Job) bool { return job.Up }))
			},
			// the jobs stop, the daemon stays up
			applied: func(t *testing.T, _ string) {
				assert.False(t, slices.ContainsFunc(daemonJobs(t), func(job run.Job) bool { return job.Up }))
				status, err := wtmDaemon().Status(t.Context())
				require.NoError(t, err)
				assert.True(t, status.Answering)
			},
		},
		{
			name:     runstart.Name,
			answers:  []domain.Answered{{Path: domain.PathWorktree, Values: []string{"main"}}, {Path: runstart.PathJob, Values: []string{"seed"}}},
			complete: true,
			// a run.toml of a task, under a daemon of the test's own at the socket the commands reach, in a home short enough for it
			setup: func(t *testing.T, main string) {
				writeRun(t, main, seedJob)
				t.Setenv("HOME", shortDir(t))
				serving(t, daemonParams{Socket: daemon.SocketPath()})
			},
			// the task runs another command since
			stale: func(t *testing.T, main string) {
				writeRun(t, main, strings.Replace(seedJob, `cmd = "true"`, `cmd = "true || true"`, 1))
			},
			untouched: func(t *testing.T, _ string) {
				assert.Empty(t, daemonJobs(t))
			},
			applied: func(t *testing.T, _ string) {
				assert.Len(t, daemonJobs(t), 1)
			},
		},
		{
			name:     runstop.Name,
			answers:  []domain.Answered{{Path: domain.PathWorktree, Values: []string{"main"}}, {Path: runstop.PathJob, Values: []string{"idle"}}},
			complete: true,
			// a service up in main, under a daemon of the test's own at the socket the commands reach, in a home short enough for it
			setup: func(t *testing.T, main string) {
				writeRun(t, main, idleJob("idle"))
				t.Setenv("HOME", shortDir(t))
				serving(t, daemonParams{Socket: daemon.SocketPath()})
				startIdle(t, main, "idle")
			},
			// the job is a stack since, whose launcher left it up: the plan stops it with its stop command
			stale: func(t *testing.T, main string) {
				client := daemon.Daemon{SocketPath: daemon.SocketPath(), Version: version}
				_, err := client.Stop(t.Context(), run.Stop{Name: "idle", WorkDir: main}, nil)
				require.NoError(t, err)
				_, err = client.Start(t.Context(), run.Spec{Job: project.Job{Name: "idle", Kind: project.Service, Cmd: "true", Stop: "true"}, WorkDir: main}, nil)
				require.NoError(t, err)
			},
			untouched: func(t *testing.T, _ string) {
				assert.True(t, slices.ContainsFunc(daemonJobs(t), func(job run.Job) bool { return job.Up }))
			},
			applied: func(t *testing.T, _ string) {
				assert.False(t, slices.ContainsFunc(daemonJobs(t), func(job run.Job) bool { return job.Up }))
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
			name:     "sync",
			answers:  []domain.Answered{{Path: domain.PathWorktree, Values: []string{"feat/s"}}},
			complete: true,
			// main moves on: feat/s, created from it, is to rebase onto it
			setup: func(t *testing.T, main string) {
				gittest.Git(t, main, "worktree", "add", "-q", "-b", "feat/s", filepath.Join(filepath.Dir(main), ".trees", "feat-s"))
				recordParent(t, main, "feat/s", "main")
				gittest.Git(t, main, "commit", "-q", "--allow-empty", "-m", "moved on")
			},
			// the parent's tip is part of the plan
			stale: func(t *testing.T, main string) {
				gittest.Git(t, main, "commit", "-q", "--allow-empty", "-m", "moved on again")
			},
			untouched: func(t *testing.T, main string) {
				assert.NotEqual(t, gittest.Output(t, main, "rev-parse", "main"), gittest.Output(t, main, "rev-parse", "feat/s"))
			},
			applied: func(t *testing.T, main string) {
				assert.Equal(t, gittest.Output(t, main, "rev-parse", "main"), gittest.Output(t, main, "rev-parse", "feat/s"))
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

// fakeGH puts first on the PATH a gh, signed in, that knows one open pull request, 7, of the branch feat/pr into main.
func fakeGH(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	pr := `{"number":7,"url":"https://github.com/acme/acme/pull/7","state":"OPEN","headRefName":"feat/pr","baseRefName":"main","isCrossRepository":false,"title":"Add a feature","isDraft":false,"author":{"login":"alice"}}`
	script := "#!/bin/sh\ncase \"$1 $2\" in\n'auth status') exit 0;;\n'pr list') echo '[" + pr + "]';;\n'pr view') echo '" + pr + "';;\n*) exit 1;;\nesac\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "gh"), []byte(script), 0o755))
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
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
var reads = []string{"list", "resolve", "status", "tree", "run-logs", "run-ps", "run-list", "daemon-status"}

// machine are the commands on wtm's daemon, the machine's one: they run on no repository, so wire.go builds none of them per repository and none refuses a directory with no wtm config.
var machine = []string{"daemon-status", "daemon-stop", "daemon-restart", "daemon-stop-jobs"}

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
		domain.JobStarted{JobChange: domain.JobChange{Worktree: domain.JobWorktree{Branch: "feat/a", Path: "/repo/.trees/feat-a"}, Job: domain.EventJob{Name: "web", Kind: project.Service, URL: "http://localhost:3100", Hold: run.HoldOwn}}},
		domain.JobStopped{JobChange: domain.JobChange{Worktree: domain.JobWorktree{Branch: "feat/a", Path: "/repo/.trees/feat-a"}, Job: domain.EventJob{Name: "web", Kind: project.Service, Hold: run.HoldOwn}}},
		domain.JobCrashed{JobChange: domain.JobChange{Worktree: domain.JobWorktree{Branch: "feat/a", Path: "/repo/.trees/feat-a"}, Job: domain.EventJob{Name: "web", Kind: project.Service, Hold: run.HoldOwn}, ExitCode: new(1), LastLines: []string{"listen EADDRINUSE"}}},
		domain.JobExited{JobChange: domain.JobChange{Worktree: domain.JobWorktree{Branch: "feat/a", Path: "/repo/.trees/feat-a"}, Job: domain.EventJob{Name: "migrate", Kind: project.Task, Hold: run.HoldOwn}, ExitCode: new(0)}},
	}
}

// idleJob is a run.toml of one service of name, which runs until it is stopped.
func idleJob(name string) string {
	return "[[job]]\nname = \"" + name + "\"\nkind = \"service\"\ncmd = \"sleep 30\"\n"
}

// startIdle starts the service of name in main through wtm's daemon the commands reach: a process of the test's own daemon, which a stop stops.
func startIdle(t *testing.T, main, name string) {
	t.Helper()
	_, err := daemon.Daemon{SocketPath: daemon.SocketPath(), Version: version}.Start(t.Context(), run.Spec{Job: project.Job{Name: name, Kind: project.Service, Cmd: "sleep 30"}, WorkDir: main}, nil)
	require.NoError(t, err)
}

// startIdleFor is startIdle, the service started for main's repository, as run up starts it.
func startIdleFor(t *testing.T, main, name string) {
	t.Helper()
	root, err := mainWorktree(t.Context(), main)
	require.NoError(t, err)
	_, err = daemon.Daemon{SocketPath: daemon.SocketPath(), Version: version}.Start(t.Context(), run.Spec{Job: project.Job{Name: name, Kind: project.Service, Cmd: "sleep 30"}, WorkDir: main, Origin: &run.Origin{Root: root, Branch: "main"}}, nil)
	require.NoError(t, err)
}

// seedJob is a run.toml of one task, which run up runs to its end.
const seedJob = "[[job]]\nname = \"seed\"\nkind = \"task\"\ncmd = \"true\"\n"
