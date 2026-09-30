package dashboard

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
)

// creating puts a background create in flight and lets it name its target, the
// way the prompter does once the wizard is answered: from there on, its hooks are
// still running on that worktree.
func creating(t *testing.T, model Model, branch string) Model {
	t.Helper()
	model, cmd := updateCmd(model, key(domain.KeyNew))
	if cmd == nil {
		t.Fatal("the create run did not start")
	}
	model, _ = model.applyFlow(opTargetMsg{id: model.ops.running[0].id, targets: []string{branch}})
	if _, held := model.ops.holding(branch); !held {
		t.Fatalf("the run does not hold %q", branch)
	}
	return model
}

func selectBranch(t *testing.T, model Model, branch string) Model {
	t.Helper()
	for index, status := range model.statuses {
		if status.Branch == branch {
			model.cursor = index
			return model
		}
	}
	t.Fatalf("no worktree %q in the list", branch)
	return model
}

// This is the case the mode declaration exists for: the create is in the
// background, its worktree is already in the list, and its hooks are still
// running on it.
func TestAWorktreeCannotBeDeletedWhileItsCreateHooksRun(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a", "feat")
	model = creating(t, model, "feat")
	model = selectBranch(t, model, "feat")

	model, cmd := model.startClean("feat")

	if cmd != nil {
		t.Fatal("the removal must be refused while the create still holds the worktree")
	}
	if kinds := runningKinds(model); kinds != domain.OpKindCreate {
		t.Fatalf("running = %q, want the create alone", kinds)
	}
	if last := lastOutput(model); !strings.Contains(last, "feat") || !strings.Contains(last, domain.OpKindCreate) {
		t.Errorf("refusal = %q, want it to name the worktree and what holds it", last)
	}
	if !model.outputExpanded {
		t.Error("a refusal nobody can see is a click that did nothing")
	}
}

func TestTheMenuShowsTheDeleteEntryAsUnusableWhileTheTargetIsHeld(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a", "feat")
	model = creating(t, model, "feat")
	model = selectBranch(t, model, "feat")

	items := menuActions(model.menuItems())
	if items[0].disabled == "" {
		t.Fatal("the entry must say it cannot be used, not fail silently once clicked")
	}

	model = update(model, key(domain.KeyMenu))
	model, cmd := updateCmd(model, namedKey(tea.KeyEnter))

	if cmd != nil || runningKinds(model) != domain.OpKindCreate {
		t.Errorf("running = %q, want the disabled entry to have started nothing", runningKinds(model))
	}
}

func TestAnotherWorktreeStaysDeletableDuringABackgroundRun(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a", "feat")
	model = creating(t, model, "feat")
	model = selectBranch(t, model, "a")

	model, cmd := model.startClean("a")

	if cmd == nil {
		t.Fatal("a background run holds its own target, not the whole dashboard")
	}
	if kinds := runningKinds(model); kinds != domain.OpKindCreate+","+domain.OpKindClean {
		t.Errorf("running = %q, want both runs", kinds)
	}
}

func TestABlockingRunHoldsEveryAction(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a", "feat")
	model, cmd := model.startClean("a")
	if cmd == nil {
		t.Fatal("the removal did not start")
	}

	model, cmd = updateCmd(model, key(domain.KeyNew))

	if cmd != nil {
		t.Fatal("a blocking run must hold back what the user starts next")
	}
	if kinds := runningKinds(model); kinds != domain.OpKindClean {
		t.Fatalf("running = %q, want the clean alone", kinds)
	}
	if last := lastOutput(model); !strings.Contains(last, domain.OpKindClean) {
		t.Errorf("refusal = %q, want it to name the run in the way", last)
	}
}

func TestAFinishedRunReleasesItsTarget(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a", "feat")
	model = creating(t, model, "feat")

	model = update(model, opDoneMsg{id: model.ops.running[0].id})

	if _, held := model.ops.holding("feat"); held {
		t.Fatal("a finished run must let go of its target")
	}
	if _, cmd := model.startClean("feat"); cmd == nil {
		t.Error("the worktree must be actionable again")
	}
}

func runningKinds(model Model) string {
	kinds := make([]string, 0, len(model.ops.running))
	for _, op := range model.ops.running {
		kinds = append(kinds, op.kind)
	}
	return strings.Join(kinds, ",")
}

func lastOutput(model Model) string {
	if len(model.outputLines) == 0 {
		return ""
	}
	return model.outputLines[len(model.outputLines)-1]
}

// A run over several worktrees locks every one of them: reading a cumulative
// answer as a single value would release the lock on all but the first.
func TestARunHoldsEveryWorktreeItActsOn(t *testing.T) {
	ops, id := operations{}.begin(operation{kind: domain.OpKindRunUp, mode: flow.ModeBackground})
	ops = ops.retarget(id, []string{"a", "b"})

	for _, branch := range []string{"a", "b"} {
		if _, held := ops.holding(branch); !held {
			t.Errorf("%q is not held by the run acting on it", branch)
		}
	}
	if _, held := ops.holding("c"); held {
		t.Error("a worktree outside the run reads as held")
	}
}

// The run flows answer with paths — the daemon's half of a job's key — and every
// reader of an operation speaks branches. Without the translation the lock is
// released the moment the recap is answered, on a run that has not even started.
func TestARunRetargetedByPathKeepsHoldingItsRow(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a", "feat")
	model, cmd := updateCmd(model, key(domain.KeyNew))
	if cmd == nil {
		t.Fatal("the create run did not start")
	}
	id := model.ops.running[0].id

	model, _ = model.applyFlow(opTargetMsg{id: id, targets: []string{"/tmp/feat"}})

	if _, held := model.ops.holding("feat"); !held {
		t.Fatalf("targets = %v, want the row's branch: a path reaches no row", model.ops.running[0].targets)
	}
}

// A worktree git cannot name is left as it is: the path is the only identity it
// has, and dropping it would release the lock instead of moving it.
func TestARunOnAnUnnamedWorktreeKeepsItsPath(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a")
	model, _ = updateCmd(model, key(domain.KeyNew))
	id := model.ops.running[0].id

	model, _ = model.applyFlow(opTargetMsg{id: id, targets: []string{"/tmp/detached"}})

	if _, held := model.ops.holding("/tmp/detached"); !held {
		t.Errorf("targets = %v, want the path kept", model.ops.running[0].targets)
	}
}

// A run over several worktrees posts a stage per worktree. One string per
// operation would show the last event received on every row it holds, whichever
// worktree it came from — three locked rows lying in chorus.
func TestEachHeldWorktreeShowsItsOwnStage(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a", "b")
	model, _ = updateCmd(model, key(domain.KeyNew))
	id := model.ops.running[0].id
	model, _ = model.applyFlow(opTargetMsg{id: id, targets: []string{"a", "b"}})

	model, _ = model.applyFlow(opStageMsg{id: id, target: "a", stage: "starting web (1/2)"})
	model, _ = model.applyFlow(opStageMsg{id: id, target: "b", stage: "migrate finished"})

	op, _ := model.ops.byID(id)
	if got := op.stageFor("a"); got != "starting web (1/2)" {
		t.Errorf("stage of a = %q, want its own", got)
	}
	if got := op.stageFor("b"); got != "migrate finished" {
		t.Errorf("stage of b = %q, want its own", got)
	}
}

// A flow acting on a single worktree says nothing about which one, and every row
// it holds shows what it did say.
func TestAStageNamingNoWorktreeShowsOnEveryHeldRow(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a", "b")
	model, _ = updateCmd(model, key(domain.KeyNew))
	id := model.ops.running[0].id
	model, _ = model.applyFlow(opTargetMsg{id: id, targets: []string{"a", "b"}})

	model, _ = model.applyFlow(opStageMsg{id: id, stage: "Rebasing"})

	op, _ := model.ops.byID(id)
	if op.stageFor("a") != "Rebasing" || op.stageFor("b") != "Rebasing" {
		t.Errorf("stages = %v, want the run's own message on both rows", op.stages)
	}
}
