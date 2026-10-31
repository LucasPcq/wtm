package cli

import (
	"context"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/next/internal/command/events"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	jobrun "github.com/LucasPcq/wtm/next/internal/domain/run"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"
	"github.com/LucasPcq/wtm/next/internal/ui/formtest"
	"github.com/LucasPcq/wtm/next/internal/ui/output"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

// A run is wtm events run from Dir over a bus that carries Lines, after failing Fail, then is interrupted unless the stream Ends by itself, on the repositories Registered, app missing its config when Absent.
type run struct {
	Dir        string
	Lines      []string
	Fail       []error
	Ends       bool
	Registered []domain.Repo
	Absent     bool
}

func runEvents(t *testing.T, r run, words ...string) formtest.Printed {
	t.Helper()
	ctx, stop := context.WithCancel(t.Context())
	defer stop()
	bus := &scriptedBus{fail: r.Fail, lines: r.Lines, stop: stop}
	if r.Ends {
		bus.stop = nil
	}
	useCase := events.UseCase{
		Bus:      bus,
		Registry: registered(r.Registered),
		Open:     opening(r.Absent),
		Now:      func() time.Time { return fixedNow },
		Recheck:  time.Hour,
	}
	return formtest.Run(ctx, func(streams *output.Streams) *cobra.Command {
		return New(streams, uicli.Env{Cwd: r.Dir, ProjectDir: r.Dir}, useCase, formtest.CommandLine)
	}, append([]string{events.Name}, words...)...)
}

// line is the line a publisher of repo writes for event.
func line(t *testing.T, repo domain.Repo, event domain.Event) string {
	t.Helper()
	data, err := domain.MarshalLine(domain.Line{V: domain.StreamVersion, TS: "2026-10-16T12:00:01Z", Repo: &repo, OpID: "op-1", Event: event})
	require.NoError(t, err)
	return string(data)
}

// changes are a line of each kind the bus carries for app, the operation's and an unknown type's among them, which a person is not shown.
func changes(t *testing.T) []string {
	t.Helper()
	feat := worktree.Identity{Branch: "feat/b", Path: "/code/.trees/feat-b", Parent: "main", Isolation: project.Isolated, CreatedAt: "2026-10-16T12:00:00Z"}
	numbered := feat
	numbered.Ordinal = new(2)
	web := domain.JobChange{Worktree: domain.JobWorktree{Branch: "feat/b", Path: "/code/.trees/feat-b"}, Job: domain.EventJob{Name: "web", Kind: project.Service, Hold: jobrun.HoldOwn}}
	crashed := web
	crashed.ExitCode, crashed.LastLines = new(1), []string{"listen EADDRINUSE"}
	return []string{
		line(t, app, domain.OperationStarted{Command: "create", Surface: "cli"}),
		line(t, app, domain.WorktreeCreated{Worktree: feat}),
		line(t, app, domain.WorktreeUpdated{Worktree: numbered, Changed: []worktree.IdentityField{worktree.IdentityOrdinal}}),
		line(t, app, domain.WorktreeChanged{Worktree: numbered, Changed: []worktree.PolledField{worktree.PolledHead, worktree.PolledJobs}}),
		line(t, app, domain.WorktreeProvisioned{Worktree: numbered, OK: true}),
		line(t, app, domain.WorktreeProvisioned{Worktree: numbered, Hook: "make setup", ExitCode: new(2)}),
		line(t, app, domain.UnitFinished{Subject: "feat/b", Status: domain.StatusDone}),
		line(t, app, domain.OperationFinished{Command: "create", Surface: "cli", OK: true}),
		line(t, app, domain.JobStarted{JobChange: web}),
		line(t, app, domain.JobCrashed{JobChange: crashed}),
		line(t, app, domain.JobStopped{JobChange: web}),
		line(t, app, domain.JobExited{JobChange: domain.JobChange{Worktree: domain.JobWorktree{Branch: "feat/b", Path: "/code/.trees/feat-b"}, Job: domain.EventJob{Name: "migrate", Kind: project.Task, Hold: jobrun.HoldOwn}, ExitCode: new(0)}}),
		`{"v":2,"type":"job.restarted","ts":"2026-10-16T12:00:02Z","repo":{"root":"/code/app","common_dir":"/code/app/.git"},"job":{"name":"web"}}`,
	}
}
