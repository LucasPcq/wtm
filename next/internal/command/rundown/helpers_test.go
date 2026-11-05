package rundown_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/rundown"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
	"github.com/LucasPcq/wtm/next/internal/service/operationtest"
	"github.com/LucasPcq/wtm/next/internal/service/runjobs"

	"github.com/stretchr/testify/require"
)

func (w *world) at(name string) string {
	if name == "main" {
		return w.main
	}
	return linkedPath(w.root, name)
}

// up lists the job of name up in the worktree of branch for itself, started under profile.
func (w *world) up(name, branch, profile string) {
	w.daemon.listed = append(w.daemon.listed, service(name, w.at(branch), profile))
}

// removedWith lists a job of name up for this repository in the worktree removed at path.
func (w *world) removedWith(name, path string) {
	left := service(name, path, "dev")
	left.Repo = w.main
	w.daemon.listed = append(w.daemon.listed, left)
}

// shared lists the shared service db running in main, held by main when mainHolds and claimed by each worktree of claims.
func (w *world) shared(mainHolds bool, claims ...string) {
	w.daemon.listed = append(w.daemon.listed, run.Job{Name: "db", WorkDir: w.main, Up: true, Kind: project.Service, State: run.Running, Hold: run.HoldOwn, SharedDir: w.main, MainHolds: mainHolds})
	for _, branch := range claims {
		w.daemon.listed = append(w.daemon.listed, run.Job{Name: "db", WorkDir: w.at(branch), Up: true, Kind: project.Service, Hold: run.HoldClaim, SharedDir: w.main, Profile: "full"})
	}
}

func (w *world) plan(t *testing.T, req rundown.Request) (rundown.Plan, []domain.Warning) {
	t.Helper()
	planned, warnings, err := w.usecase.Plan(t.Context(), req)
	require.NoError(t, err)
	return planned, warnings
}

func (w *world) refused(t *testing.T, req rundown.Request) *domain.Error {
	t.Helper()
	_, _, err := w.usecase.Plan(t.Context(), req)
	failure, ok := errors.AsType[*domain.Error](err)
	require.True(t, ok, "want a *domain.Error, got %v", err)
	return failure
}

func (w *world) apply(t *testing.T, req rundown.Request) (rundown.Result, *operationtest.Recorder) {
	t.Helper()
	return w.applyIn(t.Context(), t, req)
}

func (w *world) applyIn(ctx context.Context, t *testing.T, req rundown.Request) (rundown.Result, *operationtest.Recorder) {
	t.Helper()
	planned, _ := w.plan(t, req)
	report := &operationtest.Recorder{}
	return w.usecase.Apply(ctx, planned, report.Add), report
}

// stopping is the request to stop the jobs of names, from the main checkout.
func (w *world) stopping(names ...string) rundown.Request {
	return rundown.Request{Worktrees: names}.From(domain.Caller{Cwd: w.main})
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

func statuses(item rundown.Item) map[string]runjobs.StopStatus {
	got := map[string]runjobs.StopStatus{}
	for _, job := range item.Detail.Jobs {
		got[job.Name] = job.Status
	}
	return got
}

var errStack = &run.Error{Code: run.CodeStopFailed, Message: "the stop command of cache failed (exit 4)"}
