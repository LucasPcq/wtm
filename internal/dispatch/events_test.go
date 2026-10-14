package dispatch_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LucasPcq/wtm/internal/dispatch"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/kernel"
	"github.com/LucasPcq/wtm/internal/testutil/dispatchtest"
)

func TestAnOperationIsAnnouncedThenFinishedUnitByUnit(t *testing.T) {
	b := bench(t)
	ran := run(t.Context(), runParams{
		Bench:   b,
		Repo:    &repo{base: "main"},
		Request: request{Branches: []string{"feat/a", "feat/b"}},
		Options: dispatch.Options{Surface: dispatch.SurfaceCLI, CorrelationID: "agent-7"},
	})
	require.NoError(t, ran.Err)

	ok := true
	ts := dispatchtest.Clock.Format("2006-01-02T15:04:05Z07:00")
	operation := &domain.EventOperation{Command: commandName, Surface: string(dispatch.SurfaceCLI)}
	one, two := 1, 2
	worktree := func(branch string, ordinal *int) *domain.WorktreeIdentity {
		return &domain.WorktreeIdentity{Branch: branch, Path: "/wt/" + branch, Parent: "main", Ordinal: ordinal}
	}
	stamped := func(event domain.Event) domain.Event {
		event.TS, event.OpID, event.CorrelationID = ts, "op-1", "agent-7"
		return event
	}
	assert.Equal(t, []domain.Event{
		stamped(domain.Event{Type: domain.EventOperationStarted, Operation: operation}),
		stamped(domain.Event{Type: domain.EventWorktreeCreated, Worktree: worktree("feat/a", nil)}),
		stamped(domain.Event{Type: domain.EventWorktreeUpdated, Worktree: worktree("feat/a", &one), Changed: []domain.IdentityField{domain.IdentityOrdinal}}),
		stamped(domain.Event{Type: domain.EventWorktreeProvisioned, Worktree: worktree("feat/a", &one), OK: &ok}),
		stamped(domain.Event{Type: domain.EventUnitFinished, Unit: &domain.EventUnit{Subject: "feat/a", Status: string(kernel.StatusDone)}}),
		stamped(domain.Event{Type: domain.EventWorktreeCreated, Worktree: worktree("feat/b", nil)}),
		stamped(domain.Event{Type: domain.EventWorktreeUpdated, Worktree: worktree("feat/b", &two), Changed: []domain.IdentityField{domain.IdentityOrdinal}}),
		stamped(domain.Event{Type: domain.EventWorktreeProvisioned, Worktree: worktree("feat/b", &two), OK: &ok}),
		stamped(domain.Event{Type: domain.EventUnitFinished, Unit: &domain.EventUnit{Subject: "feat/b", Status: string(kernel.StatusDone)}}),
		stamped(domain.Event{Type: domain.EventOperationFinished, Operation: operation, OK: &ok}),
	}, b.Bus.Events())
}

func TestAWorktreeIsAnnouncedCreatedBeforeItsHookRuns(t *testing.T) {
	b := bench(t)
	var beforeHook []domain.EventType
	r := &repo{}
	r.onHook = func() { beforeHook = b.Bus.Types() }
	ran := run(t.Context(), runParams{Bench: b, Repo: r, Request: request{Branches: []string{"feat/a"}}})
	require.NoError(t, ran.Err)

	assert.Equal(t, []domain.EventType{domain.EventOperationStarted, domain.EventWorktreeCreated, domain.EventWorktreeUpdated}, beforeHook)
}

func TestAHookThatFailsIsProvisionedNotOKAndFailsTheOperation(t *testing.T) {
	b := bench(t)
	ran := run(t.Context(), runParams{Bench: b, Repo: &repo{hookErr: errors.New("exit 1")}, Request: request{Branches: []string{"feat/a"}}})
	require.NoError(t, ran.Err)

	assert.Equal(t, []domain.EventType{
		domain.EventOperationStarted, domain.EventWorktreeCreated, domain.EventWorktreeUpdated,
		domain.EventWorktreeProvisioned, domain.EventUnitFinished, domain.EventOperationFinished,
	}, b.Bus.Types())
	events := b.Bus.Events()
	provisioned, finished := events[3], events[5]
	require.NotNil(t, provisioned.OK)
	assert.False(t, *provisioned.OK)
	require.NotNil(t, finished.OK)
	assert.False(t, *finished.OK, "an item that failed fails the operation, as the command line's exit code says")
	assert.Equal(t, string(kernel.CodePhaseFailed), finished.Code)
}

func TestAnOperationThatFailsEndsWithItsCode(t *testing.T) {
	b := bench(t)
	ran := run(t.Context(), runParams{Bench: b, Repo: &repo{applyErr: errors.New("disk full")}, Request: request{Branches: []string{"feat/a"}}})
	require.Error(t, ran.Err)

	events := b.Bus.Events()
	require.Equal(t, []domain.EventType{domain.EventOperationStarted, domain.EventOperationFinished}, b.Bus.Types())
	finished := events[1]
	require.NotNil(t, finished.OK)
	assert.False(t, *finished.OK)
	assert.Equal(t, string(kernel.CodeInternal), finished.Code)
}

func TestARefusedRunPublishesNothing(t *testing.T) {
	b := bench(t)
	ran := run(t.Context(), runParams{Bench: b, Repo: &repo{}, Request: request{}})
	require.Error(t, ran.Err)

	assert.Empty(t, b.Bus.Events())
}

func TestADryRunPublishesNothing(t *testing.T) {
	b := bench(t)
	ran := run(t.Context(), runParams{Bench: b, Repo: &repo{}, Request: request{Branches: []string{"feat/a"}}, Options: dispatch.Options{DryRun: true}})
	require.NoError(t, ran.Err)

	assert.Empty(t, b.Bus.Events())
}

func TestARunStaleBeforeApplyPublishesNothing(t *testing.T) {
	b := bench(t)
	ran := run(t.Context(), runParams{Bench: b, Repo: &repo{}, Request: request{Branches: []string{"feat/a"}}, Options: dispatch.Options{ExpectFingerprint: "elsewhere"}})
	require.Error(t, ran.Err)

	assert.Empty(t, b.Bus.Events())
}
