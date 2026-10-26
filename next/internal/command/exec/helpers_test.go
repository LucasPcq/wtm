package exec_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/exec"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/operationtest"
	"github.com/stretchr/testify/require"
)

func (w world) at(name string) string {
	return linkedPath(w.root, name)
}

func (w world) record(t *testing.T, name string, meta worktree.Meta) {
	t.Helper()
	_, err := w.meta.Update(t.Context(), worktree.MetaUpdate{Branch: name, Change: func(stored *worktree.Meta) error {
		*stored = meta
		return nil
	}})
	require.NoError(t, err)
}

func (w world) questions(t *testing.T, req exec.Request) exec.Questions {
	t.Helper()
	questions, err := w.usecase.Questions(t.Context(), req)
	require.NoError(t, err)
	return questions
}

func (w world) plan(t *testing.T, req exec.Request) (exec.Plan, []domain.Warning) {
	t.Helper()
	planned, warnings, err := w.usecase.Plan(t.Context(), req)
	require.NoError(t, err)
	return planned, warnings
}

func (w world) apply(t *testing.T, req exec.Request) (exec.Result, *operationtest.Recorder) {
	t.Helper()
	return w.applyIn(t.Context(), t, req)
}

func (w world) applyIn(ctx context.Context, t *testing.T, req exec.Request) (exec.Result, *operationtest.Recorder) {
	t.Helper()
	planned, _ := w.plan(t, req)
	report := &operationtest.Recorder{}
	return w.usecase.Apply(ctx, planned, report.Add), report
}

// running asks to run command in the worktrees of names.
func running(command string, names ...string) exec.Request {
	return exec.Request{Worktrees: names, Command: command}
}

func failureOf(t *testing.T, err error) *domain.Error {
	t.Helper()
	failure, ok := errors.AsType[*domain.Error](err)
	require.True(t, ok, err)
	return failure
}

func fieldCodes(fields []domain.FieldError) map[string][]string {
	found := map[string][]string{}
	for _, field := range fields {
		found[field.Path] = append(found[field.Path], field.Code)
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

func names(targets []exec.Target) []string {
	found := make([]string, 0, len(targets))
	for _, target := range targets {
		found = append(found, target.Worktree)
	}
	return found
}

func statuses(items []exec.Item) map[string]domain.Status {
	found := map[string]domain.Status{}
	for _, item := range items {
		found[item.Subject] = item.Status
	}
	return found
}

func itemOf(t *testing.T, items []exec.Item, subject string) exec.Item {
	t.Helper()
	for _, item := range items {
		if item.Subject == subject {
			return item
		}
	}
	t.Fatalf("no item %s", subject)
	return exec.Item{}
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

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

func atoi(t *testing.T, s string) int {
	t.Helper()
	n, err := strconv.Atoi(s)
	require.NoError(t, err)
	return n
}

func splitLines(s string) []string {
	return strings.Split(strings.TrimSuffix(s, "\n"), "\n")
}

// newCancellingRecorder records the events of a run and cancels it with cancel as the first command starts.
func newCancellingRecorder(cancel context.CancelFunc) *operationtest.Recorder {
	report := &operationtest.Recorder{}
	report.On = func(event domain.Event) {
		if _, started := event.(domain.PhaseStarted); started {
			cancel()
		}
	}
	return report
}
