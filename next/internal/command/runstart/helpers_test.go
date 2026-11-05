package runstart_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/runstart"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/operationtest"
	"github.com/LucasPcq/wtm/next/internal/service/runjobs"

	"github.com/stretchr/testify/require"
)

func (w *world) at(name string) string {
	return linkedPath(w.root, name)
}

func (w *world) record(t *testing.T, name string, meta worktree.Meta) {
	t.Helper()
	_, err := w.meta.Update(t.Context(), worktree.MetaUpdate{Branch: name, Change: func(stored *worktree.Meta) error {
		*stored = meta
		return nil
	}})
	require.NoError(t, err)
}

// up lists the job of name up in the worktree of branch, as the daemon would.
func (w *world) up(name string, branch string, ports map[string]int) {
	w.daemon.listed = append(w.daemon.listed, run.Job{Name: name, WorkDir: w.pathOf(branch), Up: true, State: run.Running, Hold: run.HoldOwn, Ports: ports, URL: "http://" + name + ".up"})
}

// pathOf is the root of the worktree of branch, main's for main.
func (w *world) pathOf(branch string) string {
	if branch == "main" {
		return w.main
	}
	return w.at(branch)
}

func (w *world) questions(t *testing.T, req runstart.Request) runstart.Questions {
	t.Helper()
	questions, err := w.usecase.Questions(t.Context(), req)
	require.NoError(t, err)
	return questions
}

func (w *world) plan(t *testing.T, req runstart.Request) (runstart.Plan, []domain.Warning) {
	t.Helper()
	planned, warnings, err := w.usecase.Plan(t.Context(), req)
	require.NoError(t, err)
	return planned, warnings
}

func (w *world) refused(t *testing.T, req runstart.Request) *domain.Error {
	t.Helper()
	_, _, err := w.usecase.Plan(t.Context(), req)
	return failureOf(t, err)
}

func (w *world) apply(t *testing.T, req runstart.Request) (runstart.Result, *operationtest.Recorder) {
	t.Helper()
	return w.applyIn(t.Context(), t, req)
}

func (w *world) applyIn(ctx context.Context, t *testing.T, req runstart.Request) (runstart.Result, *operationtest.Recorder) {
	t.Helper()
	planned, _ := w.plan(t, req)
	report := &operationtest.Recorder{}
	return w.usecase.Apply(ctx, planned, report.Add), report
}

// starting asks to start job in the worktree of name.
func starting(job, name string) runstart.Request {
	return runstart.Request{Worktree: name, Job: job}
}

func failureOf(t *testing.T, err error) *domain.Error {
	t.Helper()
	failure, ok := errors.AsType[*domain.Error](err)
	require.True(t, ok, err)
	return failure
}

// statusesOf are the status of each job of item, by name.
func statusesOf(item runstart.Item) map[string]runjobs.JobStatus {
	found := map[string]runjobs.JobStatus{}
	for _, job := range item.Detail.Jobs {
		found[job.Name] = job.Status
	}
	return found
}

// specOf is the start the daemon got for the job of name in the worktree at workDir.
func (d *daemon) specOf(t *testing.T, workDir, name string) run.Spec {
	t.Helper()
	for _, spec := range d.started {
		if spec.WorkDir == workDir && spec.Job.Name == name {
			return spec
		}
	}
	t.Fatalf("no start of %s in %s", name, workDir)
	return run.Spec{}
}

func eventsOf[T domain.Event](events []domain.Event) []T {
	var found []T
	for _, event := range events {
		if e, is := event.(T); is {
			found = append(found, e)
		}
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

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}
