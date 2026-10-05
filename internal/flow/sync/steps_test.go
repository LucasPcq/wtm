package sync

import (
	"context"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
)

// testFlow installs a Prompter: session() reads Interactive() to resolve the
// parent question from the flags.
func testFlow(request Request, statuses []domain.WorktreeStatus) *syncFlow {
	return &syncFlow{runCtx: context.Background(), request: request, statuses: statuses, prompter: flow.Unattended{}}
}

var stack = []domain.WorktreeStatus{
	{Branch: "main", IsParent: true},
	{Branch: "feat-a"},
	{Branch: "feat-b", IsDirty: true},
	{Branch: "feat-c", RebaseInProgress: true},
}

func TestSelectionOptionsTagWhatSyncWouldSkip(t *testing.T) {
	content, err := testFlow(Request{}, stack).selectionStep().Build(flow.Answers{})
	if err != nil {
		t.Fatalf("build selection: %v", err)
	}

	byValue := map[string]flow.Option{}
	for _, option := range content.Options {
		byValue[option.Value] = option
	}
	if byValue["main"].Label != "main"+domain.PinnedSuffixBase {
		t.Fatalf("the base must be labelled, got %q", byValue["main"].Label)
	}
	if byValue["feat-b"].Tag != domain.SyncTagDirty {
		t.Fatalf("a dirty worktree must be tagged, got %+v", byValue["feat-b"])
	}
	if byValue["feat-c"].Tag != domain.SyncTagRebasing {
		t.Fatalf("a worktree mid-rebase must be tagged, got %+v", byValue["feat-c"])
	}
}

// The CLI never sends a Precheck: its picker opens empty, as it does today.
func TestSelectionStartsUncheckedWithoutPrecheck(t *testing.T) {
	content, _ := testFlow(Request{}, stack).selectionStep().Build(flow.Answers{})

	for _, option := range content.Options {
		if option.Selected {
			t.Fatalf("%s must not arrive checked without Precheck", option.Value)
		}
	}
}

func TestSelectionPrechecksWhatTheSurfaceAsked(t *testing.T) {
	content, _ := testFlow(Request{Precheck: []string{"feat-a", "feat-b"}}, stack).
		selectionStep().Build(flow.Answers{})

	checked := map[string]bool{}
	for _, option := range content.Options {
		checked[option.Value] = option.Selected
	}
	if !checked["feat-a"] || !checked["feat-b"] || checked["feat-c"] {
		t.Fatalf("Precheck must decide exactly what arrives checked, got %+v", checked)
	}
}

// The CLAUDE.md model for the --yes axis: never a fallback picker, an error
// naming the missing flag.
func TestSelectionResolveNamesAll(t *testing.T) {
	_, err := testFlow(Request{}, stack).selectionStep().Resolve(flow.Answers{})

	if err == nil {
		t.Fatal("an unattended run with no selection must refuse")
	}
	if !strings.Contains(err.Error(), "--"+domain.FlagAll) {
		t.Fatalf("the refusal must name --all, got: %v", err)
	}
}

func TestConflictStepDefaultsToAborting(t *testing.T) {
	answer, err := testFlow(Request{}, stack).conflictStep().Resolve(flow.Answers{})
	if err != nil {
		t.Fatalf("resolve conflict: %v", err)
	}
	if answer.Value != conflictNormal {
		t.Fatalf("the safe default aborts the rebase, got %q", answer.Value)
	}
}

// --keep-conflict reorders the options instead of answering the step: the
// question stays visible and its other outcome one keystroke away, as the picker
// had it.
func TestConflictStepLeadsWithKeepWhenFlagged(t *testing.T) {
	f := testFlow(Request{KeepConflict: true}, stack)

	if _, preset := f.session().Presets.Get(KeyConflict); preset {
		t.Fatal("--keep-conflict must not make the question disappear")
	}
	content, err := f.conflictStep().Build(flow.Answers{})
	if err != nil {
		t.Fatalf("build conflict: %v", err)
	}
	if content.Options[0].Value != conflictKeep {
		t.Fatalf("--keep-conflict must lead with keep, got %q", content.Options[0].Value)
	}
}

func TestConflictStepResolvesToKeepWhenFlagged(t *testing.T) {
	answer, err := testFlow(Request{KeepConflict: true}, stack).conflictStep().Resolve(flow.Answers{})
	if err != nil {
		t.Fatalf("resolve conflict: %v", err)
	}
	if answer.Value != conflictKeep {
		t.Fatalf("an unattended run must honour --keep-conflict, got %q", answer.Value)
	}
}

// The skip reads the selection, never a plan: no git call from a Skip.
func TestConflictStepIsSkippedForABaseOnlyRefresh(t *testing.T) {
	f := testFlow(Request{BaseBranch: "main"}, stack)

	skip, reason := f.conflictStep().Skip(flow.Answers{}.WithValues(KeySelection, []string{"main"}))
	if !skip || reason != domain.SyncNoRebaseStep {
		t.Fatalf("a base-only refresh has no conflict to have an opinion about, got %v %q", skip, reason)
	}

	skip, reason = f.conflictStep().Skip(flow.Answers{}.WithValues(KeySelection, []string{"main", "feat-a"}))
	if skip || reason != "" {
		t.Fatalf("a rebase step must be asked about, got %v %q", skip, reason)
	}
}

// The cancel row belongs to the surface (flowui and the dashboard both append
// it); declaring it here would double it.
func TestRecapLeavesTheCancelRowToTheSurface(t *testing.T) {
	f := testFlow(Request{BaseBranch: "main"}, stack)
	plan := domain.SyncPlan{BaseBranch: "main", Steps: []domain.SyncStep{{Branch: "feat-a", SourceBranch: "main"}}}

	content := f.confirmContent(plan, flow.Answers{})

	if len(content.Options) != 1 || content.Options[0].Label != domain.SyncConfirmOption {
		t.Fatalf("the recap declares its own option only, got %+v", content.Options)
	}
}

// A plan that cannot be built is an error, not an empty cascade.
func TestRecapSurfacesAPlanFailure(t *testing.T) {
	f := testFlow(Request{BaseBranch: "main"}, stack)
	f.ctx = flow.Context{ProjectDir: t.TempDir(), StateDir: t.TempDir()}

	_, err := f.confirmStep().Load(flow.Answers{}.WithValues(KeySelection, []string{"feat-a"}))
	if err == nil {
		t.Fatal("a plan that failed to build must not read as nothing to rebase")
	}
}

func TestParentsStepIsSkippedWhenNothingIsStale(t *testing.T) {
	skip, _ := testFlow(Request{}, stack).parentsStep().Skip(flow.Answers{})

	if !skip {
		t.Fatal("with no stale parent the question must not be asked")
	}
}

func TestRecapDescribesThePlan(t *testing.T) {
	f := testFlow(Request{BaseBranch: "main"}, stack)
	plan := domain.SyncPlan{
		BaseBranch:   "main",
		BaseTargeted: true,
		Steps:        []domain.SyncStep{{Branch: "feat-a", SourceBranch: "main"}},
	}

	content := f.confirmContent(plan, flow.Answers{})

	if !strings.Contains(content.Description, "feat-a") {
		t.Fatalf("the recap must carry the plan, got: %q", content.Description)
	}
}

// --all answers the selection even on a stack reduced to its base: an unset
// preset would open the picker the flag exists to skip, and the multi-select
// would then refuse a run that has a base to refresh.
func TestAllPresetsTheSelectionWhenItNamesNothing(t *testing.T) {
	baseOnly := []domain.WorktreeStatus{{Branch: "main", IsParent: true}}

	answer, preset := testFlow(Request{All: true}, baseOnly).session().Presets.Get(KeySelection)

	if !preset {
		t.Fatal("--all must answer the selection, whatever it resolves to")
	}
	if len(answer.Values) != 0 {
		t.Fatalf("nothing hangs off the base, got %+v", answer.Values)
	}
}

// The preset is what the recap reads the selection back from, so --all previews
// the resolved list rather than an empty one.
func TestAllPresetsEveryWorktreeButTheBase(t *testing.T) {
	answer, preset := testFlow(Request{All: true}, stack).session().Presets.Get(KeySelection)

	if !preset {
		t.Fatal("--all must answer the selection")
	}
	if strings.Join(answer.Values, ",") != "feat-a,feat-b,feat-c" {
		t.Fatalf("--all covers every worktree, the ones it will skip included, got %+v", answer.Values)
	}
}

// A run with neither args nor --all leaves the step open: an empty preset would
// pass for an answer and skip the picker.
func TestAnUnsetSelectionIsNotPreset(t *testing.T) {
	if _, preset := testFlow(Request{}, stack).session().Presets.Get(KeySelection); preset {
		t.Fatal("with nothing to preset the selection must stay a question")
	}
}

// --all on a base-only repository rebases nothing, so the conflict question has
// no situation to have an opinion about — the preset reads back empty, and the
// step must not take that for "every worktree".
func TestConflictStepIsSkippedWhenAllCoversNothing(t *testing.T) {
	f := testFlow(Request{All: true, BaseBranch: "main"}, []domain.WorktreeStatus{{Branch: "main", IsParent: true}})

	skip, reason := f.conflictStep().Skip(f.session().Presets)
	if !skip || reason != domain.SyncNoRebaseStep {
		t.Fatalf("nothing to rebase, yet the conflict question is put: %v %q", skip, reason)
	}
}

func TestConflictStepIsAskedWhenAllCoversAWorktree(t *testing.T) {
	f := testFlow(Request{All: true, BaseBranch: "main"}, stack)

	if skip, reason := f.conflictStep().Skip(f.session().Presets); skip {
		t.Fatalf("--all over a stack rebases, so the question stands: skipped %q", reason)
	}
}
