package ordinal_test

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/flow/ordinal"
	"github.com/LucasPcq/wtm/internal/service/worktree"
	"github.com/LucasPcq/wtm/internal/testutil/flowtest"
	"github.com/LucasPcq/wtm/internal/testutil/gittest"
)

func newContext(t *testing.T) (flow.Context, *flowtest.Recorder) {
	t.Helper()
	dir := gittest.InitRepo(t)
	gittest.Git(t, dir, "worktree", "add", "-b", "feat/a", filepath.Join(t.TempDir(), "feat-a"))
	rec := &flowtest.Recorder{}
	return flow.Context{ProjectDir: dir, StateDir: filepath.Join(dir, ".git", "wtm"), Publisher: rec}, rec
}

func TestEnsurePublishesTheAllocationOnlyOnce(t *testing.T) {
	ctx, rec := newContext(t)

	if err := ordinal.Ensure(ctx, "feat/a"); err != nil {
		t.Fatal(err)
	}
	if err := ordinal.Ensure(ctx, "feat/a"); err != nil {
		t.Fatal(err)
	}

	if !slices.Equal(rec.PublishedTypes(), []domain.EventType{domain.EventWorktreeUpdated}) {
		t.Fatalf("published %v, want one update", rec.PublishedTypes())
	}
	got := rec.Published[0]
	if !slices.Equal(got.Changed, []domain.IdentityField{domain.IdentityOrdinal}) || got.Worktree.Ordinal == nil || *got.Worktree.Ordinal != 1 {
		t.Fatalf("published %+v", got)
	}
}

func TestRetryAllocatesWhenTheServiceAsksForANumber(t *testing.T) {
	ctx, rec := newContext(t)
	calls := 0

	err := ordinal.Retry(ordinal.RetryParams{Context: ctx, Branch: "feat/a", Do: func() error {
		calls++
		if calls == 1 {
			return domain.ErrOrdinalUnallocated
		}
		return nil
	}})

	if err != nil || calls != 2 || len(rec.Published) != 1 {
		t.Fatalf("err = %v, calls = %d, published %v", err, calls, rec.PublishedTypes())
	}
}

func TestRetryLeavesAnyOtherErrorAlone(t *testing.T) {
	ctx, rec := newContext(t)
	boom := errors.New("boom")
	calls := 0

	err := ordinal.Retry(ordinal.RetryParams{Context: ctx, Branch: "feat/a", Do: func() error {
		calls++
		return boom
	}})

	if !errors.Is(err, boom) || calls != 1 || len(rec.Published) != 0 {
		t.Fatalf("err = %v, calls = %d, published %v", err, calls, rec.PublishedTypes())
	}
}

func TestBeforeHooksAllocatesNothingTheHooksWillNotRead(t *testing.T) {
	ctx, rec := newContext(t)

	ordinal.BeforeHooks(ctx, "feat/a")

	if len(rec.Published) != 0 {
		t.Fatalf("published %v for hooks that read no run variables", rec.PublishedTypes())
	}
}

func TestBeforeHooksNumbersAWorktreeWhoseHooksReadTheRunEnv(t *testing.T) {
	ctx, rec := newContext(t)
	if err := os.MkdirAll(ctx.StateDir, 0o755); err != nil {
		t.Fatal(err)
	}
	runToml := "[[job]]\nname = \"db\"\nkind = \"service\"\ncmd = \"docker compose up -d\"\nports = { DB_PORT = 5432 }\n"
	if err := os.WriteFile(filepath.Join(ctx.StateDir, domain.RunFileName), []byte(runToml), 0o644); err != nil {
		t.Fatal(err)
	}
	ref := worktree.WorktreeRef{ProjectDir: ctx.ProjectDir, StateDir: ctx.StateDir, Branch: "feat/a"}
	if err := worktree.SetIsolation(worktree.SetIsolationParams{Ref: ref, Isolation: domain.IsolationIsolated}); err != nil {
		t.Fatal(err)
	}

	ordinal.BeforeHooks(ctx, "feat/a")

	if !slices.Equal(rec.PublishedTypes(), []domain.EventType{domain.EventWorktreeUpdated}) {
		t.Fatalf("published %v, want the ordinal update", rec.PublishedTypes())
	}
}
