package runlogs_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/next/internal/adapter/joblog"
	"github.com/LucasPcq/wtm/next/internal/command/runlogs"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/run"

	"github.com/stretchr/testify/require"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

func (w *world) at(branch string) string {
	if branch == "main" {
		return w.main
	}
	return linkedPath(w.root, branch)
}

// logged writes texts as the log of job in the worktree of branch, a second apart from at.
func (w *world) logged(t *testing.T, branch, job string, texts ...string) {
	t.Helper()
	log, err := joblog.Open(run.LogFile{Dir: run.LogDir(w.stateDir, branch), Job: job})
	require.NoError(t, err)
	defer log.Close()
	log.Write(lines(texts...))
}

// lines are texts as a job prints them, a second apart from at.
func lines(texts ...string) []run.Line {
	out := make([]run.Line, 0, len(texts))
	for i, text := range texts {
		out = append(out, run.Line{At: at.Add(time.Duration(i) * time.Second), Text: text})
	}
	return out
}

func (w *world) read(t *testing.T, req runlogs.Request) runlogs.Outcome {
	t.Helper()
	read, err := w.usecase.Run(t.Context(), req, domain.RunOptions{})
	require.NoError(t, err)
	return read
}

func (w *world) refused(t *testing.T, req runlogs.Request) *domain.Error {
	t.Helper()
	_, err := w.usecase.Run(t.Context(), req, domain.RunOptions{})
	require.Error(t, err)
	failure := domain.AsError(domain.AsErrorParams{Err: err})
	require.NotNil(t, failure)
	return failure
}

// watched is what Watch streamed for req: its lines, as worktree, job and text, and its warnings' codes.
func (w *world) watched(ctx context.Context, t *testing.T, req runlogs.Request) ([]string, []string, error) {
	t.Helper()
	var got, warned []string
	err := w.usecase.Watch(ctx, req, runlogs.Out{
		Emit: func(entry runlogs.Entry) error {
			got = append(got, entry.Worktree+" "+entry.Job+" "+entry.Text)
			return nil
		},
		Absent: func(runlogs.Absent) error { return nil },
		Warn:   func(warning domain.Warning) { warned = append(warned, warning.Code) },
	})
	return got, warned, err
}

func jobOf(t *testing.T, item runlogs.Item, name string) runlogs.Job {
	t.Helper()
	require.NotNil(t, item.Detail)
	for _, job := range item.Detail.Jobs {
		if job.Name == name {
			return job
		}
	}
	require.Failf(t, "no job", "%s has no job %s", item.Subject, name)
	return runlogs.Job{}
}

func namesOf(item runlogs.Item) []string {
	var names []string
	for _, job := range item.Detail.Jobs {
		names = append(names, job.Name)
	}
	return names
}

func textsOf(job runlogs.Job) []string {
	var texts []string
	for _, line := range job.Lines {
		texts = append(texts, line.Text)
	}
	return texts
}

func codes(warnings []domain.Warning) []string {
	var out []string
	for _, warning := range warnings {
		out = append(out, warning.Code)
	}
	return out
}

func fieldCodes(fields []domain.FieldError) []string {
	var out []string
	for _, field := range fields {
		out = append(out, field.Code)
	}
	return out
}

func removeFile(path string) error {
	return os.Remove(path)
}
