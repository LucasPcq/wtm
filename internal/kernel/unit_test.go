package kernel_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/LucasPcq/wtm/internal/kernel"
)

// worktreeUnit is a unit shaped like create's: a saga that adds a worktree
// then its metadata, committed as the worktree's path, then the phases given.
func worktreeUnit(d *disk, branch string, then ...kernel.Phase[string]) kernel.Unit[string] {
	return kernel.Unit[string]{
		Saga: kernel.Saga[string]{
			Steps: []kernel.SagaStep{
				{Name: "worktree", Do: d.add(branch), Undo: d.remove(branch)},
				{Name: "meta", Do: d.add(branch + "/meta"), Undo: d.remove(branch + "/meta")},
			},
			Commit: func(context.Context) (string, error) { return "/wt/" + branch, nil },
		},
		Then: then,
	}
}

func each(ctx context.Context, params kernel.EachParams[string, string]) []kernel.Item[string] {
	params.Subject = func(branch string) string { return branch }
	return kernel.Each(ctx, params)
}

// statuses reads items back as "subject:status/reason".
func statuses(items []kernel.Item[string]) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, item.Subject+":"+string(item.Status)+"/"+string(item.Reason))
	}
	return out
}

func phase(name string, run func(context.Context, string) (string, error)) kernel.Phase[string] {
	return kernel.Phase[string]{Name: name, Run: run}
}

// — A run with nothing in its way —

func TestEachRunsOneUnitPerItemInOrder(t *testing.T) {
	d := &disk{}
	items := each(context.Background(), kernel.EachParams[string, string]{
		Items: []string{"a", "b"},
		Unit:  func(branch string) kernel.Unit[string] { return worktreeUnit(d, branch) },
	})
	if got, want := statuses(items), []string{"a:done/", "b:done/"}; !slices.Equal(got, want) {
		t.Errorf("items = %v, want %v", got, want)
	}
	if got, want := d.list(), []string{"a", "a/meta", "b", "b/meta"}; !slices.Equal(got, want) {
		t.Errorf("disk = %v, want %v", got, want)
	}
}

func TestEachWithNoItemsDoesNothing(t *testing.T) {
	items := each(context.Background(), kernel.EachParams[string, string]{
		Unit: func(string) kernel.Unit[string] { t.Fatal("a unit ran"); return kernel.Unit[string]{} },
	})
	if len(items) != 0 {
		t.Errorf("items = %+v", items)
	}
}

func TestTheCommitIsTheItemsDetailAndEachPhaseCompletesIt(t *testing.T) {
	d := &disk{}
	ports := phase("ports", func(_ context.Context, path string) (string, error) { return path + " ports=3000", nil })
	hooks := phase("hooks", func(_ context.Context, path string) (string, error) { return path + " hooks=ok", nil })
	items := each(context.Background(), kernel.EachParams[string, string]{
		Items: []string{"a"},
		Unit:  func(branch string) kernel.Unit[string] { return worktreeUnit(d, branch, ports, hooks) },
	})
	if items[0].Detail != "/wt/a ports=3000 hooks=ok" {
		t.Errorf("detail = %q", items[0].Detail)
	}
}

func TestEachUnitSaysWhatItIsDoingAsItGoes(t *testing.T) {
	progress := &recorder{}
	stopJobs := kernel.Prep{Name: "stop jobs", Run: func(context.Context) error { return nil }}
	each(context.Background(), kernel.EachParams[string, string]{
		Items: []string{"a"},
		Emit:  progress,
		Unit: func(branch string) kernel.Unit[string] {
			unit := worktreeUnit(&disk{}, branch, phase("hooks", func(_ context.Context, path string) (string, error) { return path, nil }))
			unit.Before = []kernel.Prep{stopJobs}
			return unit
		},
	})
	want := []string{
		"a unit.started",
		"a phase.started stop jobs", "a phase.finished stop jobs",
		"a phase.started worktree", "a phase.finished worktree",
		"a phase.started meta", "a phase.finished meta",
		"a phase.started hooks", "a phase.finished hooks",
		"a unit.finished done",
	}
	if got := progress.lines(); !slices.Equal(got, want) {
		t.Errorf("progress =\n%v\nwant\n%v", got, want)
	}
}

func TestAnyFunctionCanBeAnEmitter(t *testing.T) {
	var kinds []kernel.ProgressKind
	each(context.Background(), kernel.EachParams[string, string]{
		Items: []string{"a"},
		Unit:  func(string) kernel.Unit[string] { return kernel.Unit[string]{} },
		Emit:  kernel.EmitFunc(func(progress kernel.Progress) { kinds = append(kinds, progress.Kind) }),
	})
	if !slices.Equal(kinds, []kernel.ProgressKind{kernel.ProgressUnitStarted, kernel.ProgressUnitFinished}) {
		t.Errorf("kinds = %v", kinds)
	}
}

// — The saga: all or nothing —

func TestAFailedSagaIsUndoneInReverseAndLeavesNothing(t *testing.T) {
	d := &disk{}
	var undone []string
	track := func(name string, undo func(context.Context) error) func(context.Context) error {
		return func(ctx context.Context) error { undone = append(undone, name); return undo(ctx) }
	}
	items := each(context.Background(), kernel.EachParams[string, string]{
		Items: []string{"a"},
		Unit: func(string) kernel.Unit[string] {
			return kernel.Unit[string]{Saga: kernel.Saga[string]{Steps: []kernel.SagaStep{
				{Name: "worktree", Do: d.add("a"), Undo: track("worktree", d.remove("a"))},
				{Name: "meta", Do: d.add("a/meta"), Undo: track("meta", d.remove("a/meta"))},
				{Name: "publish", Do: fail},
			}}}
		},
	})
	item := items[0]
	if item.Status != kernel.StatusFailed || item.Error.Code != kernel.CodeStepFailed || item.Error.Params[kernel.ParamStep] != "publish" {
		t.Errorf("item = %+v", item)
	}
	if !errors.Is(item.Error, errBoom) {
		t.Errorf("the step's error is the cause: %v", item.Error)
	}
	if !slices.Equal(undone, []string{"meta", "worktree"}) || len(d.list()) != 0 {
		t.Errorf("undone = %v, disk = %v", undone, d.list())
	}
}

func TestAStepWithNoUndoLeavesItToAnEarlierStep(t *testing.T) {
	d := &disk{}
	items := each(context.Background(), kernel.EachParams[string, string]{
		Items: []string{"a"},
		Unit: func(string) kernel.Unit[string] {
			return kernel.Unit[string]{Saga: kernel.Saga[string]{Steps: []kernel.SagaStep{
				{Name: "worktree", Do: d.add("a"), Undo: d.remove("a")},
				{Name: "env", Do: d.add("a/.env")},
				{Name: "meta", Do: fail},
			}}}
		},
	})
	if items[0].Status != kernel.StatusFailed || !slices.Equal(d.list(), []string{"a/.env"}) {
		t.Errorf("here nothing removes a/.env: item %+v, disk %v", items[0], d.list())
	}
}

func TestACommitThatFailsUndoesEveryStep(t *testing.T) {
	d := &disk{}
	items := each(context.Background(), kernel.EachParams[string, string]{
		Items: []string{"a"},
		Unit: func(branch string) kernel.Unit[string] {
			unit := worktreeUnit(d, branch)
			unit.Saga.Commit = func(context.Context) (string, error) { return "", errBoom }
			return unit
		},
	})
	if items[0].Status != kernel.StatusFailed || items[0].Error.Params[kernel.ParamStep] != "commit" || len(d.list()) != 0 {
		t.Errorf("item = %+v, disk = %v", items[0], d.list())
	}
}

func TestAnUndoThatFailsNamesWhatIsLeftAndProposesTheFollowUp(t *testing.T) {
	d := &disk{}
	cleanIt := func(step string) *kernel.FollowUp {
		return &kernel.FollowUp{Command: "clean", Code: "test.left_behind", Params: map[string]string{kernel.ParamStep: step}}
	}
	items := each(context.Background(), kernel.EachParams[string, string]{
		Items: []string{"a"},
		Unit: func(string) kernel.Unit[string] {
			return kernel.Unit[string]{Saga: kernel.Saga[string]{
				Steps: []kernel.SagaStep{
					{Name: "worktree", Do: d.add("a"), Undo: d.remove("a")},
					{Name: "meta", Do: d.add("a/meta"), Undo: fail},
					{Name: "publish", Do: fail},
				},
				LeftBehind: cleanIt,
			}}
		},
	})
	failure := items[0].Error
	if items[0].Status != kernel.StatusFailed || failure.Code != kernel.CodeUndoFailed {
		t.Fatalf("item = %+v", items[0])
	}
	if failure.Params[kernel.ParamStep] != "meta" || failure.Params[kernel.ParamLeft] != "worktree,meta" {
		t.Errorf("params = %v", failure.Params)
	}
	if failure.FollowUp == nil || failure.FollowUp.Command != "clean" || failure.FollowUp.Params[kernel.ParamStep] != "meta" {
		t.Errorf("follow-up = %+v", failure.FollowUp)
	}
	if !d.has("a") || !d.has("a/meta") {
		t.Errorf("the walk back stops at the undo that failed: %v", d.list())
	}
}

func TestAnUndoThatFailsWithNothingToProposeHasNoFollowUp(t *testing.T) {
	items := each(context.Background(), kernel.EachParams[string, string]{
		Items: []string{"a"},
		Unit: func(string) kernel.Unit[string] {
			return kernel.Unit[string]{Saga: kernel.Saga[string]{Steps: []kernel.SagaStep{
				{Name: "worktree", Do: func(context.Context) error { return nil }, Undo: fail},
				{Name: "meta", Do: fail},
			}}}
		},
	})
	if items[0].Error.Code != kernel.CodeUndoFailed || items[0].Error.FollowUp != nil {
		t.Errorf("error = %+v", items[0].Error)
	}
}

// — Interruptions (the regression tests of LUC-257, on Each) —

func TestAnInterruptedBatchFinishesTheUnitUnderWayAndStopsThere(t *testing.T) {
	d := &disk{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	interrupt := func(context.Context) error { cancel(); return nil }
	items := each(ctx, kernel.EachParams[string, string]{
		Items: []string{"a", "b", "c"},
		Unit: func(branch string) kernel.Unit[string] {
			unit := worktreeUnit(d, branch)
			unit.Saga.Steps = append([]kernel.SagaStep{{Name: "ctrl-c", Do: interrupt}}, unit.Saga.Steps...)
			return unit
		},
	})
	if got, want := statuses(items), []string{"a:done/", "b:skipped/interrupted", "c:skipped/interrupted"}; !slices.Equal(got, want) {
		t.Errorf("items = %v, want %v", got, want)
	}
	if got, want := d.list(), []string{"a", "a/meta"}; !slices.Equal(got, want) {
		t.Errorf("the unit under way finishes whole, the next never starts: %v", got)
	}
}

func TestTheSagaRunsUnderTheShieldItIsHanded(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var taken, released int
	shield := func(ctx context.Context) (context.Context, func()) {
		taken++
		return context.WithoutCancel(ctx), func() { released++ }
	}
	var stepSawTheCancel bool
	items := each(ctx, kernel.EachParams[string, string]{
		Items:  []string{"a"},
		Shield: shield,
		Unit: func(string) kernel.Unit[string] {
			return kernel.Unit[string]{Saga: kernel.Saga[string]{Steps: []kernel.SagaStep{
				{Name: "worktree", Do: func(context.Context) error { cancel(); return nil }},
				{Name: "meta", Do: func(ctx context.Context) error { stepSawTheCancel = ctx.Err() != nil; return nil }},
			}}}
		},
	})
	if stepSawTheCancel || items[0].Status != kernel.StatusDone {
		t.Errorf("the next step ran cancelled: %v, item %+v", stepSawTheCancel, items[0])
	}
	if taken != 1 || released != 1 {
		t.Errorf("shield taken %d, released %d", taken, released)
	}
}

func TestAnInterruptInThePrepLeavesTheUnitUntouched(t *testing.T) {
	d := &disk{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stopJobs := kernel.Prep{Name: "stop jobs", Run: func(context.Context) error { cancel(); return context.Canceled }}
	items := each(ctx, kernel.EachParams[string, string]{
		Items: []string{"a"},
		Unit: func(branch string) kernel.Unit[string] {
			unit := worktreeUnit(d, branch)
			unit.Before = []kernel.Prep{stopJobs}
			return unit
		},
	})
	if got, want := statuses(items), []string{"a:cancelled/interrupted"}; !slices.Equal(got, want) {
		t.Errorf("items = %v, want %v", got, want)
	}
	if items[0].Error.Kind != kernel.KindCancelled || items[0].Error.Params[kernel.ParamPhase] != "stop jobs" {
		t.Errorf("the error names the prep it stopped in: %+v", items[0].Error)
	}
	if len(d.list()) != 0 {
		t.Errorf("nothing changed: %v", d.list())
	}
}

func TestAnInterruptBetweenThePrepAndTheSagaStartsNoSaga(t *testing.T) {
	d := &disk{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	items := each(ctx, kernel.EachParams[string, string]{
		Items: []string{"a"},
		Unit: func(branch string) kernel.Unit[string] {
			unit := worktreeUnit(d, branch)
			unit.Before = []kernel.Prep{{Name: "check", Run: func(context.Context) error { cancel(); return nil }}}
			return unit
		},
	})
	if items[0].Status != kernel.StatusCancelled || len(d.list()) != 0 {
		t.Errorf("item = %+v, disk = %v", items[0], d.list())
	}
}

func TestAFailureTheInterruptCausedCountsAsCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	hookKilled := func(context.Context) error { cancel(); return errBoom }
	items := each(ctx, kernel.EachParams[string, string]{
		Items:         []string{"a", "b", "c"},
		StopOnFailure: true,
		Unit: func(branch string) kernel.Unit[string] {
			unit := worktreeUnit(&disk{}, branch)
			unit.Before = []kernel.Prep{{Name: "on_clean", Run: hookKilled}}
			return unit
		},
	})
	if got, want := statuses(items), []string{"a:cancelled/interrupted", "b:skipped/interrupted", "c:skipped/interrupted"}; !slices.Equal(got, want) {
		t.Errorf("items = %v, want %v", got, want)
	}
}

func TestAnInterruptDuringThenKeepsTheCommitAndNamesWhatWasNotDone(t *testing.T) {
	d := &disk{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var hooksRan bool
	ports := phase("ports", func(_ context.Context, path string) (string, error) { cancel(); return path + " ports=3000", nil })
	hooks := phase("hooks", func(_ context.Context, path string) (string, error) { hooksRan = true; return path, nil })
	items := each(ctx, kernel.EachParams[string, string]{
		Items: []string{"a", "b"},
		Unit:  func(branch string) kernel.Unit[string] { return worktreeUnit(d, branch, ports, hooks) },
	})
	if got, want := statuses(items), []string{"a:cancelled/interrupted", "b:skipped/interrupted"}; !slices.Equal(got, want) {
		t.Errorf("items = %v, want %v", got, want)
	}
	if hooksRan || !d.has("a/meta") {
		t.Errorf("the saga stays, the phases after the interrupt never run: hooks %v, disk %v", hooksRan, d.list())
	}
	if items[0].Detail != "/wt/a ports=3000" || items[0].Error.Params[kernel.ParamPhase] != "hooks" {
		t.Errorf("the item keeps what was done and names the phase not run: %+v", items[0])
	}
}

// — Failures —

func TestAPrepThatFailsStopsItsUnitBeforeTheSaga(t *testing.T) {
	d := &disk{}
	items := each(context.Background(), kernel.EachParams[string, string]{
		Items: []string{"a", "b"},
		Unit: func(branch string) kernel.Unit[string] {
			unit := worktreeUnit(d, branch)
			if branch == "a" {
				unit.Before = []kernel.Prep{{Name: "on_clean", Run: fail}}
			}
			return unit
		},
	})
	if got, want := statuses(items), []string{"a:failed/", "b:done/"}; !slices.Equal(got, want) {
		t.Errorf("items = %v, want %v", got, want)
	}
	if items[0].Error.Code != kernel.CodePhaseFailed || items[0].Error.Params[kernel.ParamPhase] != "on_clean" || d.has("a") {
		t.Errorf("error = %+v, disk = %v", items[0].Error, d.list())
	}
}

func TestARefusalInThePrepKeepsItsOwnCode(t *testing.T) {
	refused := &kernel.Error{Kind: kernel.KindRefused, Code: "test.dirty"}
	items := each(context.Background(), kernel.EachParams[string, string]{
		Items: []string{"a"},
		Unit: func(string) kernel.Unit[string] {
			return kernel.Unit[string]{Before: []kernel.Prep{{Name: "check", Run: func(context.Context) error { return refused }}}}
		},
	})
	if items[0].Error != refused {
		t.Errorf("error = %+v", items[0].Error)
	}
}

func TestAPhaseThatFailsKeepsTheCommitAndStopsTheNextPhases(t *testing.T) {
	d := &disk{}
	var hooksRan bool
	ports := phase("ports", func(context.Context, string) (string, error) { return "", errBoom })
	hooks := phase("hooks", func(_ context.Context, path string) (string, error) { hooksRan = true; return path, nil })
	items := each(context.Background(), kernel.EachParams[string, string]{
		Items: []string{"a"},
		Unit:  func(branch string) kernel.Unit[string] { return worktreeUnit(d, branch, ports, hooks) },
	})
	if items[0].Status != kernel.StatusFailed || items[0].Detail != "/wt/a" || hooksRan || !d.has("a") {
		t.Errorf("item = %+v, hooks %v, disk %v", items[0], hooksRan, d.list())
	}
	if items[0].Error.Code != kernel.CodePhaseFailed || items[0].Error.Params[kernel.ParamPhase] != "ports" {
		t.Errorf("error = %+v", items[0].Error)
	}
}

func TestAFailureStopsTheBatchOnlyWhenAskedTo(t *testing.T) {
	failingA := func(d *disk) func(string) kernel.Unit[string] {
		return func(branch string) kernel.Unit[string] {
			unit := worktreeUnit(d, branch)
			if branch == "a" {
				unit.Saga.Steps[1].Do = fail
			}
			return unit
		}
	}
	stopped := each(context.Background(), kernel.EachParams[string, string]{Items: []string{"a", "b"}, StopOnFailure: true, Unit: failingA(&disk{})})
	if got, want := statuses(stopped), []string{"a:failed/", "b:skipped/not_reached"}; !slices.Equal(got, want) {
		t.Errorf("stopped = %v, want %v", got, want)
	}
	carried := each(context.Background(), kernel.EachParams[string, string]{Items: []string{"a", "b"}, Unit: failingA(&disk{})})
	if got, want := statuses(carried), []string{"a:failed/", "b:done/"}; !slices.Equal(got, want) {
		t.Errorf("carried on = %v, want %v", got, want)
	}
}
