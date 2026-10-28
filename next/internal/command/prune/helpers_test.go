package prune_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/adapter/gittest"
	"github.com/LucasPcq/wtm/next/internal/command/prune"
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

func (w world) questions(t *testing.T, req prune.Request) prune.Questions {
	t.Helper()
	questions, err := w.usecase.Questions(t.Context(), req)
	require.NoError(t, err)
	return questions
}

func (w world) plan(t *testing.T, req prune.Request) (prune.Plan, []domain.Warning) {
	t.Helper()
	planned, warnings, err := w.usecase.Plan(t.Context(), req)
	require.NoError(t, err)
	return planned, warnings
}

func (w world) apply(t *testing.T, req prune.Request) (prune.Result, *operationtest.Recorder) {
	t.Helper()
	return w.applyIn(t.Context(), t, req)
}

func (w world) applyIn(ctx context.Context, t *testing.T, req prune.Request) (prune.Result, *operationtest.Recorder) {
	t.Helper()
	planned, _ := w.plan(t, req)
	report := &operationtest.Recorder{}
	return w.usecase.Apply(ctx, planned, report.Add), report
}

// pruning asks to prune the worktrees of names.
func pruning(names ...string) prune.Request {
	return prune.Request{Worktrees: names}
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

func statuses(items []prune.Item) map[string]domain.Status {
	found := map[string]domain.Status{}
	for _, item := range items {
		found[item.Subject] = item.Status
	}
	return found
}

func itemOf(t *testing.T, items []prune.Item, subject string) prune.Item {
	t.Helper()
	at := slices.IndexFunc(items, func(item prune.Item) bool { return item.Subject == subject })
	require.GreaterOrEqual(t, at, 0, subject)
	return items[at]
}

func itemOfPlan(t *testing.T, planned prune.Plan, name string) prune.Candidate {
	t.Helper()
	at := slices.IndexFunc(planned.Worktrees, func(c prune.Candidate) bool { return c.Worktree == name })
	require.GreaterOrEqual(t, at, 0, name)
	return planned.Worktrees[at]
}

// keptOf are the worktrees the plan leaves as they are, prunedOf those it removes: a kept one has no phase.
func keptOf(planned prune.Plan) []string {
	var found []string
	for _, c := range planned.Worktrees {
		if len(c.Phases) == 0 {
			found = append(found, c.Worktree)
		}
	}
	return found
}

func prunedOf(planned prune.Plan) []string {
	var found []string
	for _, c := range planned.Worktrees {
		if len(c.Phases) > 0 {
			found = append(found, c.Worktree)
		}
	}
	return found
}

func namesOf(candidates []prune.Candidate) []string {
	found := make([]string, 0, len(candidates))
	for _, c := range candidates {
		found = append(found, c.Worktree)
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

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

func withHook(command string) string {
	return projectToml + "\n[hooks]\non_clean = [{ cmd = " + strconv.Quote(command) + " }]\n"
}

func fieldOf(form domain.Form, path string) domain.Field {
	at := slices.IndexFunc(form.Fields, func(field domain.Field) bool { return field.Spec().Path == path })
	if at < 0 {
		return nil
	}
	return form.Fields[at]
}

// rawRecord writes content as the record of name, byte for byte: one a newer wtm wrote, or one that does not decode.
func (w world) rawRecord(t *testing.T, name, content string) {
	t.Helper()
	dir, ok := worktree.MetaDir(worktree.MetaDirParams{StateDir: w.stateDir, Branch: name})
	require.True(t, ok)
	writeFile(t, filepath.Join(dir, "meta.json"), content)
}

// warningsOf are the warnings of code among warnings.
func warningsOf(warnings []domain.Warning, code string) []domain.Warning {
	return slices.DeleteFunc(slices.Clone(warnings), func(w domain.Warning) bool { return w.Code != code })
}
