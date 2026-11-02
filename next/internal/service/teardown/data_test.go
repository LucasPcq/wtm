package teardown_test

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/env"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/teardown"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestANamespaceIsDroppedFromTheMainCheckoutWithTheWorktreesEnvironmentThenItsClaimLetGo(t *testing.T) {
	w := newWorld(seed{Run: sharedDB(), Listed: up(plain)})
	facts := w.facts(t, base(), plain)

	removed, warnings, events := w.batch(t, facts, teardown.Choice{}, false, plain)

	name := "acme_" + worktree.Slug(plain)
	assert.Equal(t, []teardown.Namespace{{Job: "db", Name: name, Status: teardown.DataDrop}}, plainData(w.removal(t, facts, plain).Data))
	require.Len(t, removed, 1)
	assert.Equal(t, []teardown.Dropped{{Job: "db", Name: name, Status: teardown.NamespaceDropped}}, removed[0].Namespaces)
	assert.Empty(t, warnings)
	require.Len(t, *w.hooks, 1)
	drop := (*w.hooks)[0]
	assert.Equal(t, "dropdb", drop.Hooks[0].Cmd)
	assert.Equal(t, "/repo/main", drop.WorkDir)
	assert.Equal(t, name, drop.Env[env.NamespaceKey])
	assert.Equal(t, "5432", drop.Env["PGPORT"], "its remove reads the ports its jobs read")
	assert.Equal(t, []domain.Phase{domain.PhaseRemove, domain.PhaseDropData}, phases(events))
	assert.Equal(t, []string{"stop_all " + at(plain)}, w.daemon.calls, "its claim is let go of once the drop is made")
	assert.Equal(t, []string{run.LogDir("/state", plain)}, *w.purged)
	assert.Empty(t, w.queue.owed)
}

func TestAJobOfTheSameNameInAnotherRepositoryIsNotTheSharedServiceUp(t *testing.T) {
	w := newWorld(seed{Run: sharedDB(), Listed: []run.Job{{Name: "db", WorkDir: "/elsewhere/main", Up: true, Hold: run.HoldOwn}}})

	facts := w.facts(t, base(), plain)

	data := w.removal(t, facts, plain).Data
	require.Len(t, data, 1)
	assert.Equal(t, teardown.DataDefer, data[0].Status)
	assert.Equal(t, []string{"db"}, teardown.Down([]teardown.Removal{w.removal(t, facts, plain)}))
}

func TestANamespaceOfAServiceDownIsOwedWithWhatItsDropNeeds(t *testing.T) {
	w := newWorld(seed{Run: sharedDB()})
	facts := w.facts(t, base(), plain)

	removed, _, events := w.batch(t, facts, teardown.Choice{}, false, plain)

	name := "acme_" + worktree.Slug(plain)
	assert.Equal(t, []teardown.Dropped{{Job: "db", Name: name, Status: teardown.NamespaceDeferred, Reason: teardown.ReasonServiceDown}}, removed[0].Namespaces)
	assert.Equal(t, []string{"sweep.data_deferred"}, codes(removed[0].Warnings))
	assert.Empty(t, *w.hooks)
	assert.NotContains(t, phases(events), domain.PhaseDropData)
	require.Len(t, w.queue.owed, 1)
	owed := w.queue.owed[0]
	assert.Equal(t, "db", owed.Job)
	assert.Equal(t, worktree.Slug(plain), owed.Worktree)
	assert.Equal(t, 1, owed.Ordinal)
	assert.Equal(t, name, owed.Name)
	assert.Equal(t, "5432", owed.Env["PGPORT"], "a drop paid later runs with the environment it would have run with now")
}

func TestDroppingDataStartsTheServiceDownDropsThenStopsItAgain(t *testing.T) {
	w := newWorld(seed{Run: sharedDB()})
	facts := w.facts(t, base(), plain)

	removed, warnings, _ := w.batch(t, facts, teardown.Choice{DropData: true}, false, plain)

	assert.Equal(t, teardown.DataStart, w.removal(t, facts, plain).With(teardown.Choice{DropData: true}).Data[0].Status)
	assert.Equal(t, teardown.NamespaceDropped, removed[0].Namespaces[0].Status)
	assert.Empty(t, warnings)
	assert.Equal(t, []string{"start db /repo/main", "stop_all " + at(plain), "stop db /repo/main"}, w.daemon.calls)
	require.Len(t, *w.hooks, 1)
}

func TestAServiceThatWillNotStartOwesItsNamespace(t *testing.T) {
	w := newWorld(seed{Run: sharedDB()})
	w.daemon.startErr = errRefused
	facts := w.facts(t, base(), plain)

	removed, _, _ := w.batch(t, facts, teardown.Choice{DropData: true}, false, plain)

	assert.Equal(t, teardown.NamespaceDeferred, removed[0].Namespaces[0].Status)
	assert.Equal(t, []string{"sweep.data_deferred"}, codes(removed[0].Warnings))
	assert.Len(t, w.queue.owed, 1)
	assert.Empty(t, *w.hooks)
}

func TestADropThatFailsIsOwedAndWarned(t *testing.T) {
	w := newWorld(seed{Run: sharedDB(), Listed: up(plain), RunHooks: hookFails})
	facts := w.facts(t, base(), plain)

	removed, _, _ := w.batch(t, facts, teardown.Choice{}, false, plain)

	assert.Equal(t, domain.StatusDone, removed[0].Status, "a drop that fails fails no removal")
	assert.Equal(t, teardown.NamespaceFailed, removed[0].Namespaces[0].Status)
	assert.Equal(t, []string{"sweep.drop_failed"}, codes(removed[0].Warnings))
	assert.Len(t, w.queue.owed, 1)
}

func TestKeepingDataKeepsEveryNamespace(t *testing.T) {
	w := newWorld(seed{Run: sharedDB(), Listed: up(plain)})
	facts := w.facts(t, base(), plain)

	removed, _, _ := w.batch(t, facts, teardown.Choice{KeepData: true}, false, plain)

	assert.Equal(t, []teardown.Dropped{{Job: "db", Name: "acme_" + worktree.Slug(plain), Status: teardown.NamespaceKept, Reason: teardown.ReasonKeepData}}, removed[0].Namespaces)
	assert.Equal(t, []string{"sweep.data_kept"}, codes(removed[0].Warnings))
	assert.Empty(t, *w.hooks)
	assert.Empty(t, w.queue.owed)
}

func TestANamespaceAnotherWorktreeReachesUnderTheSameNameIsKept(t *testing.T) {
	shared := sharedDB()
	shared.Jobs[0].Namespace.Name = "acme"
	w := newWorld(seed{Run: shared, Listed: up(plain)})

	facts := w.facts(t, base(), plain)

	data := w.removal(t, facts, plain).Data
	require.Len(t, data, 1)
	assert.Equal(t, teardown.DataKeep, data[0].Status)
	assert.Equal(t, teardown.ReasonSharedName, data[0].Reason)
}

func TestTheWorktreesOwnJobsAreStoppedBeforeItsHooksAndItsClaimsAfterItsDrops(t *testing.T) {
	w := newWorld(seed{Run: sharedDB(), Listed: append(up(plain), run.Job{Name: "web", WorkDir: at(plain), Up: true, Hold: run.HoldOwn})})
	facts := w.facts(t, onClean(), plain)

	removed, _, events := w.batch(t, facts, teardown.Choice{}, false, plain)

	assert.Equal(t, domain.StatusDone, removed[0].Status)
	assert.Equal(t, []domain.Phase{domain.PhaseStopJobs, domain.PhaseOnClean, domain.PhaseRemove, domain.PhaseDropData}, phases(events))
	assert.Equal(t, []string{"stop web " + at(plain), "stop_all " + at(plain)}, w.daemon.calls)
}

func TestAJobThatWillNotStopKeepsTheWorktreeUnlessForced(t *testing.T) {
	for _, force := range []bool{false, true} {
		w := newWorld(seed{Listed: []run.Job{{Name: "web", WorkDir: at(plain), Up: true, Hold: run.HoldOwn}}})
		w.daemon.stuck = []string{"web"}
		facts := w.facts(t, base(), plain)

		removed, _, _ := w.batch(t, facts, teardown.Choice{}, force, plain)

		if force {
			assert.Equal(t, domain.StatusDone, removed[0].Status)
			assert.Equal(t, []string{"sweep.jobs_left"}, codes(removed[0].Warnings))
			continue
		}
		assert.Equal(t, domain.StatusFailed, removed[0].Status)
		assert.Equal(t, "sweep.stop_failed", removed[0].Err.Code)
		assert.False(t, removed[0].Gone)
		assert.NotContains(t, w.git.Calls(), "RemoveWorktree "+at(plain))
	}
}

func TestJobsTheDaemonCannotTellFailTheRemovalUnlessForced(t *testing.T) {
	w := newWorld(seed{})
	w.daemon.jobsErr = run.ErrUnknown
	facts := w.facts(t, base(), plain)
	facts.JobsErr = run.ErrUnknown
	facts.Settle(w.worktrees(t, facts, plain), nil, nil)

	removed, _, _ := w.batch(t, facts, teardown.Choice{}, false, plain)

	assert.Equal(t, []domain.Phase{domain.PhaseStopJobs, domain.PhaseRemove}, w.removal(t, facts, plain).Phases)
	assert.Equal(t, "sweep.stop_failed", removed[0].Err.Code)
}

func TestADropOwedIsPaidOnceItsServiceIsUpWithTheEnvironmentItKept(t *testing.T) {
	owed := env.OwedDrop{Job: "db", Worktree: "feat-gone", Ordinal: 7, Name: "acme_feat-gone", Env: map[string]string{env.NamespaceKey: "acme_feat-gone", "PGPORT": "5439"}}
	w := newWorld(seed{Run: sharedDB(), Listed: up(plain), Owed: []env.OwedDrop{owed}})

	paid := w.service.Pay(t.Context(), w.payParams(t))

	assert.Equal(t, []string{"sweep.owed_paid"}, codes(paid.Warnings))
	assert.Empty(t, paid.Owed)
	require.Len(t, *w.hooks, 1)
	assert.Equal(t, owed.Env, (*w.hooks)[0].Env)
	assert.Equal(t, "/repo/main", (*w.hooks)[0].WorkDir)
	assert.Empty(t, w.queue.owed)
}

func TestADropTheLegacyWtmOwedIsPaidFromItsSlugAndOrdinal(t *testing.T) {
	w := newWorld(seed{Run: sharedDB(), Listed: up(plain), Owed: []env.OwedDrop{{Job: "db", Worktree: "feat-gone", Ordinal: 7}}})

	paid := w.service.Pay(t.Context(), w.payParams(t))

	assert.Equal(t, []string{"sweep.owed_paid"}, codes(paid.Warnings))
	require.Len(t, *w.hooks, 1)
	assert.Equal(t, "acme_feat-gone", (*w.hooks)[0].Env[env.NamespaceKey])
}

func TestADebtIsWithdrawnOnlyWhenAWorktreeReachesTheSameName(t *testing.T) {
	ordinal := sharedDB()
	ordinal.Jobs[0].Namespace.Name = "acme_{ordinal}"
	slug := worktree.Slug(plain)
	w := newWorld(seed{Run: ordinal, Listed: up(plain), Owed: []env.OwedDrop{
		{Job: "db", Worktree: slug, Ordinal: 9, Name: "acme_9"},
		{Job: "db", Worktree: slug, Ordinal: 1, Name: "acme_1"},
	}})

	paid := w.service.Pay(t.Context(), w.payParams(t))

	assert.ElementsMatch(t, []string{"sweep.owed_paid", "sweep.owed_withdrawn"}, codes(paid.Warnings), "feat/plain, ordinal 1, reaches acme_1 only")
	require.Len(t, *w.hooks, 1)
	assert.Equal(t, "acme_9", (*w.hooks)[0].Env[env.NamespaceKey])
	assert.Empty(t, w.queue.owed)
}

func TestADropOwedToAServiceDownStaysOwed(t *testing.T) {
	w := newWorld(seed{Run: sharedDB(), Owed: []env.OwedDrop{{Job: "db", Worktree: "feat-gone", Ordinal: 7}}})

	paid := w.service.Pay(t.Context(), w.payParams(t))

	assert.Empty(t, paid.Warnings)
	assert.Equal(t, []string{"sweep.owed"}, codes(paid.Owed))
	assert.Len(t, w.queue.owed, 1)
}

func TestAFailedDropOwedStaysOwed(t *testing.T) {
	w := newWorld(seed{Run: sharedDB(), Listed: up(plain), Owed: []env.OwedDrop{{Job: "db", Worktree: "feat-gone", Ordinal: 7}}, RunHooks: hookFails})

	paid := w.service.Pay(t.Context(), w.payParams(t))

	assert.Equal(t, []string{"sweep.drop_failed"}, codes(paid.Warnings))
	assert.Equal(t, []string{"sweep.owed"}, codes(paid.Owed))
	assert.Len(t, w.queue.owed, 1)
}

func TestAClaimThatCannotBeLetGoOfIsWarned(t *testing.T) {
	w := newWorld(seed{Run: sharedDB(), Listed: up(plain)})
	w.daemon.stopErr = errRefused
	facts := w.facts(t, base(), plain)

	removed, warnings, _ := w.batch(t, facts, teardown.Choice{}, false, plain)

	assert.Equal(t, domain.StatusDone, removed[0].Status)
	assert.Equal(t, []string{"sweep.release_failed"}, codes(warnings))
}
