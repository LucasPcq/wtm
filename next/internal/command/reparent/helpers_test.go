package reparent_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/adapter/oplock"
	"github.com/LucasPcq/wtm/next/internal/command/reparent"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/stretchr/testify/require"
)

func (w world) at(name string) string {
	return linkedPath(w.root, name)
}

func (w world) record(t *testing.T, branch string, meta worktree.Meta) {
	t.Helper()
	_, err := w.meta.Update(t.Context(), worktree.MetaUpdate{Branch: branch, Change: func(stored *worktree.Meta) error {
		*stored = meta
		return nil
	}})
	require.NoError(t, err)
}

// recordParent rewrites the parent the record of branch names, keeping the rest.
func (w world) recordParent(t *testing.T, branch, parent string) {
	t.Helper()
	_, err := w.meta.Update(t.Context(), worktree.MetaUpdate{Branch: branch, Change: func(stored *worktree.Meta) error {
		stored.SourceBranch = parent
		return nil
	}})
	require.NoError(t, err)
}

func (w world) parentOf(t *testing.T, branch string) string {
	t.Helper()
	meta, err := w.meta.Load(branch)
	require.NoError(t, err)
	return meta.SourceBranch
}

func (w world) facts(t *testing.T, req reparent.Request) reparent.Facts {
	t.Helper()
	facts, err := w.usecase.Observe(t.Context(), req)
	require.NoError(t, err)
	return facts
}

func (w world) questions(t *testing.T, req reparent.Request) reparent.Questions {
	t.Helper()
	return reparent.NewQuestions(w.facts(t, req), req)
}

// moving asks to move the worktrees of names onto to.
func moving(to string, names ...string) reparent.Request {
	return reparent.Request{Worktrees: names, To: to}
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

func valuesOf(options []domain.Option) []string {
	found := make([]string, 0, len(options))
	for _, option := range options {
		found = append(found, option.Value)
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

// unwritable makes the directory of the record of branch read-only until the test ends, so the record cannot be written.
func (w world) unwritable(t *testing.T, branch string) {
	t.Helper()
	dir, ok := worktree.MetaDir(worktree.MetaDirParams{StateDir: w.stateDir, Branch: branch})
	require.True(t, ok)
	require.NoError(t, os.Chmod(dir, 0o500))
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
}

func (w world) recordPath(t *testing.T, branch string) string {
	t.Helper()
	dir, ok := worktree.MetaDir(worktree.MetaDirParams{StateDir: w.stateDir, Branch: branch})
	require.True(t, ok)
	return filepath.Join(dir, "meta.json")
}

// lock holds the worktree lock of name exclusive for another operation of this process, as a concurrent clean of it would.
func (w world) lock(t *testing.T, name string) (*oplock.Held, error) {
	t.Helper()
	return oplock.Locks{StateDir: w.stateDir}.Take(domain.Operation{ID: "other", PID: os.Getpid(), Command: "clean", Locks: []domain.Lock{{Scope: domain.ScopeWorktree, Name: name, Mode: domain.LockExclusive}}})
}
