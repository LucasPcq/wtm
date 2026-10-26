package teardown_test

import (
	"context"
	"slices"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/teardown"
	"github.com/stretchr/testify/require"
)

// facts are the world as a command observes it, under config, its records read and the worktrees of names checked.
func (w world) facts(t *testing.T, config project.Config, names ...string) teardown.Facts {
	t.Helper()
	snapshot, err := w.git.Snapshot(t.Context())
	require.NoError(t, err)
	facts := teardown.Facts{Repository: worktree.Repository{Config: config, Snapshot: snapshot, MainPath: "/repo/main"}, Records: map[string]worktree.Meta{}}
	for _, w2 := range snapshot.Worktrees {
		if record, err := w.store.Load(w2.Branch()); err == nil {
			facts.Records[w2.Branch()] = record
		}
	}
	facts.Checks, err = w.service.Read(t.Context(), w.worktrees(t, facts, names...))
	require.NoError(t, err)
	return facts
}

func (w world) worktrees(t *testing.T, facts teardown.Facts, names ...string) []worktree.Worktree {
	t.Helper()
	var found []worktree.Worktree
	for _, name := range names {
		at := slices.IndexFunc(facts.Worktrees, func(wt worktree.Worktree) bool { return wt.Name() == name })
		require.GreaterOrEqual(t, at, 0, name)
		found = append(found, facts.Worktrees[at])
	}
	return found
}

func (w world) removal(t *testing.T, facts teardown.Facts, name string) teardown.Removal {
	t.Helper()
	return facts.Removal(w.worktrees(t, facts, name)[0], 0)
}

func (w world) remove(t *testing.T, facts teardown.Facts, name string) (teardown.Removed, []domain.Event) {
	t.Helper()
	return w.removeIn(t.Context(), t, facts, name)
}

func (w world) removeIn(ctx context.Context, t *testing.T, facts teardown.Facts, name string) (teardown.Removed, []domain.Event) {
	t.Helper()
	var events []domain.Event
	removed := w.service.Remove(ctx, teardown.RemoveParams{Command: command, Facts: facts, Removal: w.removal(t, facts, name), Report: func(e domain.Event) { events = append(events, e) }})
	return removed, events
}

func base() project.Config {
	return project.Config{Worktrees: project.Worktrees{BaseBranch: "main"}}
}

func codes(warnings []domain.Warning) []string {
	found := make([]string, 0, len(warnings))
	for _, warning := range warnings {
		found = append(found, warning.Code)
	}
	return found
}
