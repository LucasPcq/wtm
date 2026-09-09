package dashboard

import (
	"io"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	reparentflow "github.com/LucasPcq/wtm/internal/flow/reparent"
	"github.com/LucasPcq/wtm/internal/rules"
)

// The create command itself is not run here: startCreate hands the flow to a
// bubbletea command, and this asserts what the dashboard does before and around
// it. What the flow asks is covered by the modal tests.
func TestNewKeyStartsACreateRun(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a")

	model, cmd := updateCmd(model, key(domain.KeyNew))

	if cmd == nil {
		t.Fatal("n must start the create flow")
	}
	if len(model.ops.running) != 1 {
		t.Fatalf("running = %+v, want the run recorded", model.ops.running)
	}
	if got := model.ops.running[0]; got.kind != domain.OpKindCreate || got.mode != flow.ModeBackground {
		t.Errorf("operation = %+v, want the mode create declares", got)
	}
	if !model.outputExpanded {
		t.Error("a run the user cannot watch is a run they cannot trust: the output panel must open")
	}
}

func TestFinishedRunReleasesItsSlotAndReportsAFailure(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a")
	model, _ = updateCmd(model, key(domain.KeyNew))
	id := model.ops.running[0].id

	model = update(model, opDoneMsg{id: id, err: domain.ErrWorktreeExists})

	if len(model.ops.running) != 0 {
		t.Fatalf("running = %+v, want the finished run released", model.ops.running)
	}
	if len(model.outputLines) == 0 {
		t.Fatal("a failed run must say so in the output panel")
	}
}

func TestAbortedRunSaysNothingExtra(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a")
	model, _ = updateCmd(model, key(domain.KeyNew))

	model = update(model, opDoneMsg{id: model.ops.running[0].id, err: domain.ErrUserAborted})

	if len(model.outputLines) != 0 {
		t.Errorf("output = %q, want nothing: the flow already noticed the abort", model.outputLines)
	}
}

// The hooks write from the flow's goroutine while the run is still going: the
// panel must fill as they print, not once they are done.
func TestHooksStreamIntoTheOutputPanelLineByLine(t *testing.T) {
	msgs := make(chan tea.Msg, 16)
	p := presenter{send: func(msg tea.Msg) { msgs <- msg }}

	err := p.HookPhase(flow.HookPhaseParams{
		Title: domain.HooksTitleOnCreate,
		Run: func(sink flow.HookSink) error {
			if _, writeErr := io.WriteString(sink.Output, "installing\npar"); writeErr != nil {
				return writeErr
			}
			// Title line + its opStageMsg (the locked row's progress) + the
			// first line already out.
			if len(msgs) != 3 {
				t.Errorf("%d messages posted mid-run, want the title, its stage and the first line already out", len(msgs))
			}
			_, writeErr := io.WriteString(sink.Output, "tial tail")
			return writeErr
		},
	})
	if err != nil {
		t.Fatalf("hook phase: %v", err)
	}

	var lines []string
	for len(msgs) > 0 {
		switch msg := (<-msgs).(type) {
		case OutputLineMsg:
			lines = append(lines, msg.Text)
		case opStageMsg:
			if msg.stage != domain.HooksTitleOnCreate {
				t.Errorf("stage = %q, want the hook title", msg.stage)
			}
		default:
			t.Fatalf("the hook sink posted something unexpected: %#v", msg)
		}
	}

	want := []string{domain.HooksTitleOnCreate, "installing", "partial tail"}
	if len(lines) != len(want) {
		t.Fatalf("%d lines posted, want %d", len(lines), len(want))
	}
	for index, expected := range want {
		if lines[index] != expected {
			t.Errorf("line %d = %q, want %q", index, lines[index], expected)
		}
	}
}

func TestASuccessfulCreateSelectsTheNewWorktree(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a", "b")

	model, cmd := model.applyFlow(createdMsg{branch: "c"})
	if cmd == nil {
		t.Fatal("a finished create must refresh the list")
	}
	if model.cursor != 0 {
		t.Fatal("the selection only moves once the list holds the new worktree")
	}

	model = update(model, worktreesMsg{statuses: statuses("a", "b", "c"), parents: map[string]string{}})

	if model.cursor != 2 {
		t.Errorf("cursor = %d, want the new worktree selected", model.cursor)
	}
	selected, _ := model.selected()
	if selected.Branch != "c" {
		t.Errorf("selected %q, want the worktree that was just created", selected.Branch)
	}
}

func TestAFlowMessageIsDeliveredThroughTheChannelAndRearmsTheListener(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a")

	model.msgs <- OutputLineMsg{Text: "from the flow"}
	msg, ok := listenCmd(model.msgs)().(flowMsg)
	if !ok {
		t.Fatalf("listenCmd returned %T, want a flowMsg wrapping what was posted", msg)
	}

	model, cmd := updateCmd(model, msg)

	if len(model.outputLines) != 1 || model.outputLines[0] != "from the flow" {
		t.Errorf("output = %q, want the posted line", model.outputLines)
	}
	if cmd == nil {
		t.Error("handling a flow message must re-arm the listener, or the next one is never read")
	}
}

func TestBackingOutOfAQuestionLeavesNothingInTheOutputPanel(t *testing.T) {
	msgs := make(chan tea.Msg, 4)
	p := presenter{send: func(msg tea.Msg) { msgs <- msg }}

	p.Notice(flow.AbortedNotice)
	if len(msgs) != 0 {
		t.Errorf("an abort posted %d lines, want none: the modal closing already said it", len(msgs))
	}

	p.Notice(flow.Notice{Kind: flow.NoticeWarning, Text: domain.CleanCannotCleanParent})
	if len(msgs) != 1 {
		t.Errorf("a refusal posted %d lines, want it reported", len(msgs))
	}
}

func TestAnAbortedRunLeavesTheOutputPanelUntouched(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a")
	model, _ = updateCmd(model, key(domain.KeyNew))

	presenter{send: func(msg tea.Msg) { model = update(model, msg) }}.Notice(flow.AbortedNotice)
	model = update(model, opDoneMsg{id: model.ops.running[0].id, err: domain.ErrUserAborted})

	if len(model.outputLines) != 0 {
		t.Errorf("output = %q, want a cancelled run to leave no trace", model.outputLines)
	}
}

// sudo prompts for a password on the terminal, which this surface is holding:
// the run must never be told it may escalate.
func TestTheDashboardNeverOffersThePrivilegedRemoval(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "feat")

	model, cmd := model.startClean("feat")
	if cmd == nil {
		t.Fatal("the removal did not start")
	}

	model = update(model, opDoneMsg{id: model.ops.running[0].id, err: domain.ErrWorktreeRemoveFailed})

	last := model.outputLines[len(model.outputLines)-1]
	if !strings.Contains(last, "--force") || !strings.Contains(last, "feat") {
		t.Errorf("last line = %q, want the way out named: the CLI can hand sudo the terminal", last)
	}
}

// A reparent asks before it writes, so it holds the whole surface — and it holds
// the worktree the menu named, which the flow itself never gets to declare.
func TestReparentHoldsTheSurfaceAndItsWorktree(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a", "b")

	model, cmd := model.startReparent("b")

	if cmd == nil {
		t.Fatal("startReparent must hand the run to a bubbletea command")
	}
	if len(model.ops.running) != 1 {
		t.Fatalf("running = %+v, want the run recorded", model.ops.running)
	}
	got := model.ops.running[0]
	if got.kind != domain.OpKindReparent || got.mode != flow.ModeBlocking {
		t.Errorf("operation = %+v, want the mode reparent declares", got)
	}
	if got.firstTarget() != "b" {
		t.Errorf("target = %q, want the worktree the menu named", got.firstTarget())
	}
}

func TestReparentIsRefusedWhileSomethingHoldsTheWorktree(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a", "b")
	model.ops, _ = model.ops.begin(operation{kind: domain.OpKindCreate, targets: []string{"b"}})

	refused, cmd := model.startReparent("b")

	if cmd != nil {
		t.Fatal("nothing may act on a worktree another run is holding")
	}
	if len(refused.outputLines) == 0 {
		t.Error("the refusal must be stated where the user is looking")
	}
}

// The parent metadata is what the "from <x>" line of a row reads, so the list has
// to be re-read once it changes.
func TestAReparentedRunRefreshesTheList(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a", "b")

	_, cmd := model.applyFlow(reparentedMsg{})

	if cmd == nil {
		t.Error("a finished reparent must trigger a reload")
	}
}

func TestReparentPresenterStatesEveryMoveAndHowToApplyIt(t *testing.T) {
	msgs := make(chan tea.Msg, 8)
	p := reparentPresenter{presenter{send: func(msg tea.Msg) { msgs <- msg }}}

	if err := p.Reparented(reparentflow.Outcome{Results: []domain.ReparentResult{
		{Branch: "b", OldParent: "feat", NewParent: "main"},
	}}); err != nil {
		t.Fatalf("Reparented: %v", err)
	}

	var lines []string
	for len(msgs) > 0 {
		if line, ok := (<-msgs).(OutputLineMsg); ok {
			lines = append(lines, line.Text)
		}
	}
	joined := strings.Join(lines, "\n")
	for _, want := range []string{"b", "feat", "main", "wtm sync"} {
		if !strings.Contains(joined, want) {
			t.Errorf("output missing %q:\n%s", want, joined)
		}
	}
}

// The batch run asks which worktrees to move before it asks where to, so it
// starts with nothing preset and nothing to lock: it blocks the whole surface
// for its run, which is what keeps anything else from starting.
func TestBatchReparentBlocksTheSurfaceAndPresetsNothing(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a", "b")

	model, cmd := model.startBatchReparent()

	if cmd == nil {
		t.Fatal("startBatchReparent must hand the run to a bubbletea command")
	}
	if len(model.ops.running) != 1 {
		t.Fatalf("running = %+v, want the run recorded", model.ops.running)
	}
	got := model.ops.running[0]
	if got.kind != domain.OpKindReparent || got.mode != flow.ModeBlocking {
		t.Errorf("operation = %+v, want the mode reparent declares", got)
	}
	if got.firstTarget() != "" {
		t.Errorf("target = %q, want none: the worktrees are chosen inside the run", got.firstTarget())
	}
}

func TestBatchReparentIsRefusedWhileARunHoldsTheSurface(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a", "b")
	model.ops, _ = model.ops.begin(operation{kind: domain.OpKindClean, mode: flow.ModeBlocking})

	refused, cmd := model.startBatchReparent()

	if cmd != nil {
		t.Fatal("nothing may start while a blocking run holds the dashboard")
	}
	if len(refused.outputLines) == 0 {
		t.Error("the refusal must be stated where the user is looking")
	}
}

// The forest the subtree pre-check needs is derived from what the model already
// holds — the statuses and their recorded parents — rather than re-read from
// disk on a keystroke.
func TestTheModelDerivesItsForestFromWhatItAlreadyHolds(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight)
	model = update(model, worktreesMsg{
		statuses: []domain.WorktreeStatus{{Branch: "main", IsParent: true}, {Branch: "a"}, {Branch: "b"}},
		parents:  map[string]string{"a": "main", "b": "a"},
	})

	nodes := model.worktreeNodes()

	if len(nodes) != 3 {
		t.Fatalf("nodes = %+v, want one per worktree", nodes)
	}
	if nodes[2].Branch != "b" || nodes[2].SourceBranch != "a" {
		t.Errorf("node = %+v, want the recorded parent carried over", nodes[2])
	}
	if !nodes[0].IsMain {
		t.Error("the base worktree must stay the root of the forest")
	}
}

// "Sync this worktree" means the chain it hangs off: replaying it onto a parent
// nobody refreshed is the stale-parent problem, so the ancestry arrives checked —
// base first — and what hangs under it does not.
func TestSyncFromARowPreChecksItsAncestryAndNotItsChildren(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight)
	model = update(model, worktreesMsg{
		statuses: []domain.WorktreeStatus{{Branch: "main", IsParent: true}, {Branch: "a"}, {Branch: "b"}, {Branch: "c"}},
		parents:  map[string]string{"a": "main", "b": "a", "c": "main"},
	})

	got := model.ancestryOf("b")

	if len(got) != 3 || got[0] != "main" || got[1] != "a" || got[2] != "b" {
		t.Errorf("pre-checked = %v, want [main a b]", got)
	}
	// The same gesture on the parent stops there: the child is its own row.
	if parent := model.ancestryOf("a"); len(parent) != 2 || parent[1] != "a" {
		t.Errorf("pre-checked from the parent = %v, want [main a]", parent)
	}
}

// The full run offers every worktree, base included, but leaves out what a cascade
// would skip: they stay listed, with the tag that says why, and the user can still
// check them.
func TestSyncAllLeavesTheUnsyncableUnchecked(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight)
	model = update(model, worktreesMsg{
		statuses: []domain.WorktreeStatus{
			{Branch: "main", IsParent: true},
			{Branch: "clean"},
			{Branch: "dirty", IsDirty: true},
			{Branch: "stuck", RebaseInProgress: true},
		},
		parents: map[string]string{},
	})

	got := model.syncReadyBranches()

	if len(got) != 2 || got[0] != "main" || got[1] != "clean" {
		t.Errorf("pre-checked = %v, want [main clean]", got)
	}
}

// A cascade asks before it writes and rebases several worktrees, so it holds the
// whole surface and locks none of them in particular.
func TestSyncBlocksTheSurfaceAndLocksNoWorktree(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a", "b")

	model, cmd := model.startSync("b")

	if cmd == nil {
		t.Fatal("startSync must hand the run to a bubbletea command")
	}
	if len(model.ops.running) != 1 {
		t.Fatalf("running = %+v, want the run recorded", model.ops.running)
	}
	got := model.ops.running[0]
	if got.kind != domain.OpKindSync || got.mode != flow.ModeBlocking {
		t.Errorf("operation = %+v, want the mode sync declares", got)
	}
	if got.firstTarget() != "" {
		t.Errorf("target = %q, want none: the cascade covers several worktrees", got.firstTarget())
	}
	if !model.outputExpanded {
		t.Error("a run the user cannot watch is a run they cannot trust: the output panel must open")
	}
}

func TestSyncIsRefusedWhileSomethingHoldsTheWorktree(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a", "b")
	model.ops, _ = model.ops.begin(operation{kind: domain.OpKindCreate, targets: []string{"b"}})

	refused, cmd := model.startSync("b")

	if cmd != nil {
		t.Fatal("nothing may rebase a worktree another run is holding")
	}
	if len(refused.outputLines) == 0 {
		t.Error("the refusal must be stated where the user is looking")
	}
}

// A cascade rewrites branches and can move the base, so the whole list has to be
// re-read rather than one row patched.
func TestAFinishedSyncRefreshesTheList(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a", "b")

	_, cmd := model.applyFlow(syncedMsg{})

	if cmd == nil {
		t.Error("a finished sync must trigger a reload")
	}
}

// A cascade that ends on a conflict reports every step as it goes and then fails:
// the panel already holds what happened, so the run adds nothing under it.
func TestARunThatAlreadyReportedItselfAddsNoRedundantFailureLine(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a", "b")
	model, _ = model.startSync("b")

	model = update(model, opDoneMsg{id: model.ops.running[0].id, err: domain.ErrAborted})

	if len(model.ops.running) != 0 {
		t.Fatalf("running = %+v, want the finished run released", model.ops.running)
	}
	if len(model.outputLines) != 0 {
		t.Errorf("output = %q, want nothing: the steps already said what happened", model.outputLines)
	}
}

// The fast-forward is a context-menu entry: it acts on the row it hangs off, not
// on whatever the config calls the base.
func TestTheRowFastForwardActsOnTheRowItWasOpenedFrom(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight)
	model = update(model, worktreesMsg{
		statuses: []domain.WorktreeStatus{{Branch: "trunk", IsParent: true}},
		parents:  map[string]string{},
	})
	model.ops, _ = model.ops.begin(operation{kind: domain.OpKindCreate, targets: []string{"trunk"}})

	refused, cmd := model.startFastForward("trunk")

	if cmd != nil {
		t.Fatal("the run must be refused: it acts on a worktree another run is holding")
	}
	if len(refused.outputLines) == 0 {
		t.Error("the refusal must be stated where the user is looking")
	}
}

func TestTheFastForwardKeyStartsTheRunOnTheSelectedRow(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight)
	model = update(model, worktreesMsg{
		statuses: []domain.WorktreeStatus{{Branch: "feat", OriginState: domain.DivergenceBehind, OriginBehind: 2}},
		parents:  map[string]string{},
	})

	started, cmd := updateCmd(model, key(domain.KeyFastForward))

	if cmd == nil {
		t.Fatal("f must start the fast-forward flow")
	}
	if len(started.ops.running) != 1 || started.ops.running[0].kind != domain.OpKindFastForward {
		t.Fatalf("running = %+v, want the fast-forward run recorded", started.ops.running)
	}
	if started.ops.running[0].mode != flow.ModeBlocking {
		t.Errorf("mode = %v, want the mode the flow declares", started.ops.running[0].mode)
	}
}

// The batch run holds the whole surface, so it locks no single worktree, and it
// arrives with exactly the branches the badges call behind already checked.
func TestBatchFastForwardBlocksTheSurfaceAndPrechecksTheBehindOnes(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight)
	model = update(model, worktreesMsg{
		statuses: []domain.WorktreeStatus{
			{Branch: "behind", OriginState: domain.DivergenceBehind, OriginBehind: 2},
			{Branch: "current", OriginState: domain.DivergenceUpToDate},
			{Branch: "split", OriginState: domain.DivergenceDiverged, OriginAhead: 1, OriginBehind: 1},
		},
		parents: map[string]string{},
	})

	started, cmd := model.startFastForwardAll()

	if cmd == nil {
		t.Fatal("the batch entry must start the run")
	}
	if len(started.ops.running) != 1 || len(started.ops.running[0].targets) != 0 {
		t.Fatalf("running = %+v, want a run holding the surface and no worktree", started.ops.running)
	}
	precheck := rules.FastForwardReadyBranches(model.statuses)
	if len(precheck) != 1 || precheck[0] != "behind" {
		t.Fatalf("precheck = %v, want [behind]", precheck)
	}
}
