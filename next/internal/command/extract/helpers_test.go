package extract_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/extract"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/stretchr/testify/require"
)

type recorder struct {
	mu     sync.Mutex
	events []domain.Event
}

func (r *recorder) add(event domain.Event) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, event)
}

func (r *recorder) Open(domain.Operation) domain.Report {
	return r.add
}

func (r *recorder) all() []domain.Event {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.events)
}

// at is where the seeded worktree of name lives; path is where a new one would be created.
func (w world) at(name string) string {
	return linkedPath(w.root, name)
}

func (w world) path(name string) string {
	return filepath.Join(w.trees, worktree.PathSegment(name))
}

func (w world) change(t *testing.T, name, file, content string) {
	t.Helper()
	writeFile(t, filepath.Join(w.at(name), file), content)
}

func (w world) record(t *testing.T, name string, meta worktree.Meta) {
	t.Helper()
	_, err := w.meta.Update(t.Context(), worktree.MetaUpdate{Branch: name, Change: func(stored *worktree.Meta) error {
		*stored = meta
		return nil
	}})
	require.NoError(t, err)
}

func (w world) facts(t *testing.T, req extract.Request) extract.Facts {
	t.Helper()
	facts, err := w.usecase.Observe(t.Context(), req)
	require.NoError(t, err)
	return facts
}

func (w world) questions(t *testing.T, req extract.Request) extract.Questions {
	t.Helper()
	return extract.Form(w.facts(t, req), req)
}

func (w world) plan(t *testing.T, req extract.Request) (extract.Plan, []domain.Warning) {
	t.Helper()
	planned, warnings, err := w.usecase.Plan(t.Context(), req)
	require.NoError(t, err)
	return planned, warnings
}

func (w world) apply(t *testing.T, req extract.Request) (extract.Result, *recorder) {
	t.Helper()
	return w.applyIn(t.Context(), t, req)
}

func (w world) applyIn(ctx context.Context, t *testing.T, req extract.Request) (extract.Result, *recorder) {
	t.Helper()
	planned, _ := w.plan(t, req)
	report := &recorder{}
	return w.usecase.Apply(ctx, planned, report.add), report
}

// moving asks to move files from the source worktree to the other one.
func moving(files ...string) extract.Request {
	return extract.Request{Source: source, Files: files, To: other}
}

func problems(err error) map[string]string {
	found := map[string]string{}
	invalid, ok := errors.AsType[*domain.Error](err)
	if !ok {
		return found
	}
	for _, field := range invalid.Fields {
		found[field.Path] = field.Code
	}
	return found
}

func failureOf(t *testing.T, err error) *domain.Error {
	t.Helper()
	failure, ok := errors.AsType[*domain.Error](err)
	require.True(t, ok, err)
	return failure
}

func codes(warnings []domain.Warning) []string {
	found := make([]string, 0, len(warnings))
	for _, warning := range warnings {
		found = append(found, warning.Code)
	}
	return found
}

func containsKind[T domain.Event](events []domain.Event) bool {
	return slices.ContainsFunc(events, func(event domain.Event) bool {
		_, is := event.(T)
		return is
	})
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

func jsonOf(t *testing.T, value any) string {
	t.Helper()
	data, err := json.Marshal(value)
	require.NoError(t, err)
	return string(data)
}

func fieldCodes(fields []domain.FieldError) map[string]string {
	found := map[string]string{}
	for _, field := range fields {
		found[field.Path] = field.Code
	}
	return found
}

func valuesOf(options []domain.Option) []string {
	found := make([]string, 0, len(options))
	for _, option := range options {
		found = append(found, option.Value)
	}
	return found
}

func removeFile(path string) error {
	return os.Remove(path)
}
