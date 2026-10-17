package create_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/next/internal/adapter/gittest"
	"github.com/LucasPcq/wtm/next/internal/app/create"
	"github.com/LucasPcq/wtm/next/internal/wtm"
	"github.com/stretchr/testify/require"
)

type recorder struct {
	mu     sync.Mutex
	events []wtm.Event
}

func (r *recorder) add(event wtm.Event) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, event)
}

func (r *recorder) all() []wtm.Event {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.events)
}

func (w world) facts(t *testing.T, branches ...string) create.Facts {
	t.Helper()
	facts, err := w.deps.Observe(t.Context(), branches)
	require.NoError(t, err)
	return facts
}

func (w world) questions(t *testing.T, req create.Request) create.Questions {
	t.Helper()
	return create.Form(w.facts(t, req.Branches...), req)
}

func (w world) plan(t *testing.T, req create.Request) (create.Planned, []wtm.Warning) {
	t.Helper()
	facts := w.facts(t, req.Branches...)
	resolved, err := create.Resolve(facts, req)
	require.NoError(t, err)
	planned, warnings, err := create.Plan(facts, resolved)
	require.NoError(t, err)
	return planned, warnings
}

func (w world) apply(t *testing.T, req create.Request) (create.Result, *recorder) {
	t.Helper()
	return w.applyIn(t.Context(), t, req)
}

func (w world) applyIn(ctx context.Context, t *testing.T, req create.Request) (create.Result, *recorder) {
	t.Helper()
	facts := w.facts(t, req.Branches...)
	resolved, err := create.Resolve(facts, req)
	require.NoError(t, err)
	planned, _, err := create.Plan(facts, resolved)
	require.NoError(t, err)
	report := &recorder{}
	return create.Apply(ctx, create.ApplyParams{Deps: w.deps, Facts: facts, Plan: planned, FastForward: *resolved.FastForward, Report: report.add}), report
}

func problems(err error) map[string]string {
	found := map[string]string{}
	invalid, ok := errors.AsType[*wtm.Error](err)
	if !ok {
		return found
	}
	for _, field := range invalid.Fields {
		found[field.Path] = field.Code
	}
	return found
}

func statuses(items []create.Item) map[string]wtm.Status {
	found := map[string]wtm.Status{}
	for _, item := range items {
		found[item.Subject] = item.Status
	}
	return found
}

func codes(warnings []wtm.Warning) []string {
	found := make([]string, 0, len(warnings))
	for _, warning := range warnings {
		found = append(found, warning.Code)
	}
	return found
}

func values(options []create.Option) []string {
	found := make([]string, 0, len(options))
	for _, option := range options {
		found = append(found, option.Value)
	}
	return found
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(content)
}

func withHook(command string) string {
	return strings.Replace(projectToml, "on_create = []", "on_create = [{ cmd = "+strconv.Quote(command)+" }]", 1)
}

func appears(path string) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)
		for range time.Tick(10 * time.Millisecond) {
			if _, err := os.Stat(path); err == nil {
				return
			}
		}
	}()
	return done
}

func gitAddWorktree(t *testing.T, w world, name, path string) {
	t.Helper()
	gittest.Git(t, w.main, "worktree", "add", "-q", "-b", name, path)
}

func problemsOf(questions create.Questions) map[string]string {
	found := map[string]string{}
	for _, field := range questions.Errors {
		found[field.Path] = field.Code
	}
	return found
}

func behindOrigin(t *testing.T, w world, name string) {
	t.Helper()
	ahead := gittest.Output(t, w.main, "commit-tree", name+"^{tree}", "-p", name, "-m", "upstream "+name)
	gittest.Git(t, w.main, "update-ref", "refs/remotes/origin/"+name, ahead)
}

func jsonOf(t *testing.T, value any) string {
	t.Helper()
	data, err := json.Marshal(value)
	require.NoError(t, err)
	return string(data)
}
