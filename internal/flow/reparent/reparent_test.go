package reparent

import (
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/service/worktree"
	"github.com/LucasPcq/wtm/internal/testutil/flowtest"
	"github.com/LucasPcq/wtm/internal/testutil/gittest"
)

type recorder struct {
	flowtest.Recorder
	outcome Outcome
}

func (r *recorder) Reparented(outcome Outcome) error { r.outcome = outcome; return nil }

func testContext(t *testing.T) flow.Context {
	t.Helper()
	dir := gittest.InitRepo(t)
	config := domain.Config{}
	config.Project.Worktrees.BasePath = filepath.Join(t.TempDir(), "trees")
	config.Project.Worktrees.BaseBranch = "main"
	config.Project.Env.Strategy = domain.EnvStrategyExample
	return flow.Context{ProjectDir: dir, StateDir: filepath.Join(dir, ".git", "wtm"), Config: config}
}

// stack builds main ← feat ← dev-a through the service, so the fixture does not
// depend on the create flow.
func stack(t *testing.T, ctx flow.Context) {
	t.Helper()
	for _, step := range []struct{ branch, from string }{{"feat", "main"}, {"dev-a", "feat"}} {
		if _, err := worktree.Create(t.Context(), domain.CreateParams{
			ProjectDir:   ctx.ProjectDir,
			StateDir:     ctx.StateDir,
			Branch:       step.branch,
			FromBranch:   step.from,
			SourceBranch: step.from,
			Config:       ctx.Config,
			SkipHooks:    true,
		}); err != nil {
			t.Fatalf("create %s: %v", step.branch, err)
		}
	}
}

func TestRunAsksOnlyWhatTheRequestDoesNotCarry(t *testing.T) {
	ctx := testContext(t)
	stack(t, ctx)

	prompter := &flowtest.ScriptedPrompter{Answers: map[string]string{KeyRecap: confirmReparent}}
	presenter := &recorder{}

	outcome, err := Run(t.Context(), Params{
		Context:   ctx,
		Request:   Request{Branches: []string{"dev-a"}, To: "main"},
		Prompter:  prompter,
		Presenter: presenter,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := prompter.AskedKeys(); got != KeyRecap {
		t.Errorf("asked = %q, want the recap alone", got)
	}
	if len(outcome.Results) != 1 || outcome.Results[0].NewParent != "main" {
		t.Fatalf("outcome = %+v, want dev-a moved onto main", outcome)
	}
	if outcome.Results[0].OldParent != "feat" {
		t.Errorf("old parent = %q, want feat", outcome.Results[0].OldParent)
	}
}

// A flag must never make a line disappear from the recap: the worktree and the
// parent came from the request here, and both must still be read back.
func TestRecapNamesEveryMoveEvenWhenFlagsAnsweredThem(t *testing.T) {
	ctx := testContext(t)
	stack(t, ctx)

	prompter := &flowtest.ScriptedPrompter{Answers: map[string]string{KeyRecap: confirmReparent}}
	if _, err := Run(t.Context(), Params{
		Context:   ctx,
		Request:   Request{Branches: []string{"dev-a"}, To: "main"},
		Prompter:  prompter,
		Presenter: &recorder{},
	}); err != nil {
		t.Fatalf("Run: %v", err)
	}

	recap := prompter.Content[KeyRecap].Description
	for _, want := range []string{"dev-a", "feat", "main"} {
		if !strings.Contains(recap, want) {
			t.Errorf("recap missing %q:\n%s", want, recap)
		}
	}
}

func TestRunAsksTheParentWhenTheRequestHasNone(t *testing.T) {
	ctx := testContext(t)
	stack(t, ctx)

	prompter := &flowtest.ScriptedPrompter{Answers: map[string]string{
		KeyParent: "main",
		KeyRecap:  confirmReparent,
	}}

	if _, err := Run(t.Context(), Params{
		Context:   ctx,
		Request:   Request{Branches: []string{"dev-a"}},
		Prompter:  prompter,
		Presenter: &recorder{},
	}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := prompter.AskedKeys(); got != KeyParent+","+KeyRecap {
		t.Errorf("asked = %q, want the parent then the recap", got)
	}
	if desc := prompter.Content[KeyParent].Description; !strings.Contains(desc, "feat") {
		t.Errorf("the parent step should name what dev-a is rebased onto today: %q", desc)
	}
}

func TestRunAsksTheWorktreesWhenTheRequestHasNone(t *testing.T) {
	ctx := testContext(t)
	stack(t, ctx)

	prompter := &flowtest.ScriptedPrompter{
		Sets:    map[string][]string{KeyBranches: {"dev-a", "feat"}},
		Answers: map[string]string{KeyParent: "main", KeyRecap: confirmReparent},
	}

	outcome, err := Run(t.Context(), Params{
		Context:   ctx,
		Request:   Request{},
		Prompter:  prompter,
		Presenter: &recorder{},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := prompter.AskedKeys(); got != KeyBranches+","+KeyParent+","+KeyRecap {
		t.Errorf("asked = %q, want all three steps", got)
	}
	if len(outcome.Results) != 2 {
		t.Fatalf("outcome = %+v, want both worktrees reparented", outcome)
	}
	// The main worktree has no recorded parent to change, so it is never offered.
	for _, option := range prompter.Content[KeyBranches].Options {
		if option.Value == "main" {
			t.Error("the main worktree must not be listed as reparentable")
		}
	}
}

func TestRunAbortsWithoutTouchingAnything(t *testing.T) {
	ctx := testContext(t)
	stack(t, ctx)

	presenter := &recorder{}
	outcome, err := Run(t.Context(), Params{
		Context:   ctx,
		Request:   Request{Branches: []string{"dev-a"}, To: "main"},
		Prompter:  &flowtest.ScriptedPrompter{Abort: true},
		Presenter: presenter,
	})
	if err != nil {
		t.Fatalf("an abort is not an error: %v", err)
	}
	if !outcome.Aborted {
		t.Error("outcome should report the abort")
	}
	if len(presenter.Stages) != 0 {
		t.Errorf("nothing may run after an abort, got stages %v", presenter.Stages)
	}
	if got := parentOf(t, ctx, "dev-a"); got != "feat" {
		t.Errorf("dev-a parent = %q, want it untouched", got)
	}
}

func parentOf(t *testing.T, ctx flow.Context, branchName string) string {
	t.Helper()
	nodes, err := worktree.Nodes(t.Context(), worktree.NodesParams{ProjectDir: ctx.ProjectDir, StateDir: ctx.StateDir})
	if err != nil {
		t.Fatalf("nodes: %v", err)
	}
	for _, node := range nodes {
		if node.Branch == branchName {
			return node.SourceBranch
		}
	}
	t.Fatalf("no node for %s", branchName)
	return ""
}

// The unattended path is the bypass taxonomy: each required selection refuses
// while naming what would have answered it, and never falls back to a picker.
func TestUnattendedRefusesWithoutWorktrees(t *testing.T) {
	ctx := testContext(t)
	stack(t, ctx)

	_, err := Run(t.Context(), Params{
		Context:   ctx,
		Request:   Request{To: "main"},
		Prompter:  flow.Unattended{},
		Presenter: &recorder{},
	})
	if !errors.Is(err, domain.ErrReparentBranchesRequired) {
		t.Fatalf("err = %v, want the worktree selection to refuse", err)
	}
}

func TestUnattendedRefusesWithoutParent(t *testing.T) {
	ctx := testContext(t)
	stack(t, ctx)

	_, err := Run(t.Context(), Params{
		Context:   ctx,
		Request:   Request{Branches: []string{"dev-a"}},
		Prompter:  flow.Unattended{},
		Presenter: &recorder{},
	})
	if !errors.Is(err, domain.ErrReparentParentRequired) {
		t.Fatalf("err = %v, want the parent selection to refuse", err)
	}
	if !strings.Contains(err.Error(), "--"+domain.FlagTo) {
		t.Errorf("err = %v, should name --%s", err, domain.FlagTo)
	}
}

func TestUnattendedProceedsWhenBothAreGiven(t *testing.T) {
	ctx := testContext(t)
	stack(t, ctx)

	presenter := &recorder{}
	outcome, err := Run(t.Context(), Params{
		Context:   ctx,
		Request:   Request{Branches: []string{"dev-a"}, To: "main"},
		Prompter:  flow.Unattended{},
		Presenter: presenter,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(outcome.Results) != 1 {
		t.Fatalf("outcome = %+v, want the move applied with no prompt", outcome)
	}
	if presenter.outcome.Results == nil {
		t.Error("the presenter must be handed the conclusion")
	}
}

// A repeated argument is one worktree: it is validated, rewritten and reported once.
func TestRunCollapsesRepeatedBranches(t *testing.T) {
	ctx := testContext(t)
	stack(t, ctx)

	outcome, err := Run(t.Context(), Params{
		Context:   ctx,
		Request:   Request{Branches: []string{"dev-a", "dev-a"}, To: "main"},
		Prompter:  flow.Unattended{},
		Presenter: &recorder{},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(outcome.Results) != 1 {
		t.Errorf("results = %+v, want one per worktree", outcome.Results)
	}
}

func TestOperationBlocksTheSurfaceItRunsOn(t *testing.T) {
	op := Operation()
	if op.Kind != domain.OpKindReparent {
		t.Errorf("kind = %q, want %q", op.Kind, domain.OpKindReparent)
	}
	if op.Mode != flow.ModeBlocking {
		t.Error("reparent asks before it writes, so it holds the surface until answered")
	}
	if op.TargetKey != "" {
		t.Error("a reparent may hold several worktrees, so it names no single target")
	}
}

// The parent step must exclude the worktrees being moved — including when the
// selection was made in the picker rather than passed as an argument. The picker
// offering a worktree as its own parent is a choice ReparentBatch then refuses,
// which is exactly what narrowing exists to prevent.
func TestParentStepExcludesASelectionMadeInThePicker(t *testing.T) {
	ctx := testContext(t)
	stack(t, ctx)

	prompter := &flowtest.ScriptedPrompter{
		Sets:    map[string][]string{KeyBranches: {"dev-a"}},
		Answers: map[string]string{KeyParent: "main", KeyRecap: confirmReparent},
	}

	if _, err := Run(t.Context(), Params{
		Context:   ctx,
		Request:   Request{},
		Prompter:  prompter,
		Presenter: &recorder{},
	}); err != nil {
		t.Fatalf("Run: %v", err)
	}

	excluded := prompter.Content[KeyParent].ExcludeBranches
	if !slices.Contains(excluded, "dev-a") {
		t.Errorf("excluded = %v, want the selected worktree kept out of its own parent list", excluded)
	}
}

// The same narrowing when the selection came from the command line.
func TestParentStepExcludesAPresetSelection(t *testing.T) {
	ctx := testContext(t)
	stack(t, ctx)

	prompter := &flowtest.ScriptedPrompter{
		Answers: map[string]string{KeyParent: "main", KeyRecap: confirmReparent},
	}

	if _, err := Run(t.Context(), Params{
		Context:   ctx,
		Request:   Request{Branches: []string{"feat"}},
		Prompter:  prompter,
		Presenter: &recorder{},
	}); err != nil {
		t.Fatalf("Run: %v", err)
	}

	excluded := prompter.Content[KeyParent].ExcludeBranches
	if !slices.Contains(excluded, "feat") {
		t.Errorf("excluded = %v, want the preset worktree excluded", excluded)
	}
	// dev-a hangs off feat, so making it feat's parent would close a cycle.
	if !slices.Contains(excluded, "dev-a") {
		t.Errorf("excluded = %v, want the descendants excluded too", excluded)
	}
}

// Reparenting nothing is not a run: the step refuses to advance on an empty set
// rather than reaching ReparentBatch with no branches.
func TestTheWorktreeStepRefusesAnEmptySelection(t *testing.T) {
	ctx := testContext(t)
	stack(t, ctx)

	_, err := Run(t.Context(), Params{
		Context:   ctx,
		Request:   Request{},
		Prompter:  &flowtest.ScriptedPrompter{Sets: map[string][]string{KeyBranches: {}}},
		Presenter: &recorder{},
	})
	if err == nil {
		t.Fatal("an empty selection must not advance")
	}
	if !strings.Contains(err.Error(), domain.ReparentSelectAtLeastOne) {
		t.Errorf("err = %v, want it to say a worktree must be selected", err)
	}
}

func TestEachMovedWorktreeIsPublished(t *testing.T) {
	ctx := testContext(t)
	stack(t, ctx)
	presenter := &recorder{}
	ctx.Publisher = &presenter.Recorder

	if _, err := Run(t.Context(), Params{
		Context:   ctx,
		Request:   Request{Branches: []string{"dev-a"}, To: "main"},
		Prompter:  &flowtest.ScriptedPrompter{Answers: map[string]string{KeyRecap: confirmReparent}},
		Presenter: presenter,
	}); err != nil {
		t.Fatalf("Run: %v", err)
	}

	got := presenter.Published
	if len(got) != 1 || got[0].Type != domain.EventWorktreeReparented || got[0].FromParent != "feat" || got[0].Worktree.Parent != "main" {
		t.Fatalf("published %+v, want dev-a reparented from feat to main", got)
	}
}
