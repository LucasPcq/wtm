package clean_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/adapter/gittest"
	"github.com/LucasPcq/wtm/next/internal/command/clean"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/operationtest"
	"github.com/stretchr/testify/require"
)

var (
	errBranchHeld = errors.New("branch held")
	errPermission = errors.New("permission denied")
	errBroken     = errors.New("bad object")
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

func (w world) facts(t *testing.T, req clean.Request) clean.Facts {
	t.Helper()
	facts, err := w.usecase.Observe(t.Context(), req)
	require.NoError(t, err)
	return facts
}

func (w world) questions(t *testing.T, req clean.Request) clean.Questions {
	t.Helper()
	return clean.NewQuestions(w.facts(t, req), req)
}

func (w world) plan(t *testing.T, req clean.Request) (clean.Plan, []domain.Warning) {
	t.Helper()
	planned, warnings, err := w.usecase.Plan(t.Context(), req)
	require.NoError(t, err)
	return planned, warnings
}

func (w world) apply(t *testing.T, req clean.Request) (clean.Result, *operationtest.Recorder) {
	t.Helper()
	return w.applyIn(t.Context(), t, req)
}

func (w world) applyIn(ctx context.Context, t *testing.T, req clean.Request) (clean.Result, *operationtest.Recorder) {
	t.Helper()
	planned, _ := w.plan(t, req)
	report := &operationtest.Recorder{}
	return w.usecase.Apply(ctx, planned, report.Add), report
}

// removing asks to remove the worktrees of names.
func removing(names ...string) clean.Request {
	return clean.Request{Worktrees: names}
}

// gone reports whether the worktree of name, its branch and its record are all gone.
func (w world) gone(t *testing.T, name string) bool {
	t.Helper()
	_, statErr := os.Stat(w.at(name))
	_, metaErr := w.meta.Load(name)
	return errors.Is(statErr, os.ErrNotExist) && errors.Is(metaErr, os.ErrNotExist) && gittest.Output(t, w.main, "branch", "--list", name) == ""
}

// stands reports whether the worktree of name, its branch and its record are all there.
func (w world) stands(t *testing.T, name string) bool {
	t.Helper()
	_, statErr := os.Stat(w.at(name))
	_, metaErr := w.meta.Load(name)
	return statErr == nil && metaErr == nil && gittest.Output(t, w.main, "branch", "--list", name) != ""
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

func statuses(items []clean.Item) map[string]domain.Status {
	found := map[string]domain.Status{}
	for _, item := range items {
		found[item.Subject] = item.Status
	}
	return found
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

func valuesOf(options []domain.Option) []string {
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

func withHook(command string) string {
	return projectToml + "\n[hooks]\non_clean = [{ cmd = " + strconv.Quote(command) + " }]\n"
}

func skipOf(form domain.Form, path string) domain.Skip {
	at := slices.IndexFunc(form.Fields, func(field domain.Field) bool { return field.Spec().Path == path })
	if at < 0 {
		return ""
	}
	return form.Fields[at].Spec().Skip
}

// rawRecord writes content as the record of name, byte for byte: one a newer wtm wrote, or one that does not decode.
func (w world) rawRecord(t *testing.T, name, content string) {
	t.Helper()
	dir, ok := worktree.MetaDir(worktree.MetaDirParams{StateDir: w.stateDir, Branch: name})
	require.True(t, ok)
	writeFile(t, filepath.Join(dir, "meta.json"), content)
}
