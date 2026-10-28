package exec_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
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
	planned.Op = runs.next()
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

// runs names each apply's operation, as Run would, so each one has a log of its own.
var runs = &counter{}

type counter struct {
	mu sync.Mutex
	n  int
}

func (c *counter) next() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.n++
	return "op-" + strconv.Itoa(c.n)
}

// logsOf are the logs a worktree's runs left, by name.
func (w world) logsOf(t *testing.T, name string) []string {
	t.Helper()
	entries, err := os.ReadDir(w.logDir(name))
	require.NoError(t, err)
	var found []string
	for _, entry := range entries {
		found = append(found, entry.Name())
	}
	return found
}

// logDir is where the logs of the runs in the worktree of name go.
func (w world) logDir(name string) string {
	return filepath.Join(w.stateDir, "exec", worktree.StateSegment(name))
}

var errNoLog = errors.New("permission denied")

// logOf is the log the phase of the worktree of subject named.
func logOf(started []domain.PhaseStarted, subject string) string {
	for _, phase := range started {
		if phase.Subject == subject {
			return phase.Log
		}
	}
	return ""
}

// rawRecord writes content as the record of name, byte for byte: one that does not decode, or one a newer wtm wrote.
func (w world) rawRecord(t *testing.T, name, content string) {
	t.Helper()
	dir, ok := worktree.MetaDir(worktree.MetaDirParams{StateDir: w.stateDir, Branch: name})
	require.True(t, ok)
	writeFile(t, filepath.Join(dir, "meta.json"), content)
}
