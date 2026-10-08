package prune

import (
	"context"
	"sort"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/flow/orphans"
	"github.com/LucasPcq/wtm/internal/flow/run/owed"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/testutil/flowtest"
)

// The questions a prune asks are exercised against a plan handed in directly:
// the classification that produces it is covered by rules/prune_test.go, and the
// orchestration around it by the CLI characterization tests.

func flowWith(plan domain.PrunePlan, request Request) *pruneFlow {
	if request.BaseBranch == "" {
		request.BaseBranch = "main"
	}
	return &pruneFlow{runCtx: context.Background(), request: request, plan: plan}
}

func candidate(branch, reason, unsafe, parent string) domain.PruneCandidate {
	return domain.PruneCandidate{
		Branch:       branch,
		Path:         "/wt/" + branch,
		Reason:       reason,
		UnsafeReason: unsafe,
		SourceBranch: parent,
	}
}

func ask(t *testing.T, f *pruneFlow, prompter *flowtest.ScriptedPrompter) flow.Answers {
	t.Helper()
	answers, err := prompter.Ask(f.session())
	if err != nil {
		t.Fatalf("ask: %v", err)
	}
	return answers
}

// A candidate that is safe opens checked; one held back by a refusal opens
// unchecked and tagged with what holds it back. Losing either would turn a
// confirmation into a re-selection, or hide why a worktree is not going.
func TestSelectionPreChecksOnlyTheSafeCandidates(t *testing.T) {
	f := flowWith(domain.PrunePlan{Selected: []domain.PruneCandidate{
		candidate("merged-wt", domain.PruneReasonPRMerged, "", "main"),
		candidate("dirty-wt", domain.PruneReasonGone, domain.PruneSkipDirty, "main"),
	}}, Request{})

	options := f.candidateOptions()

	if len(options) != 2 {
		t.Fatalf("options = %d, want one per candidate", len(options))
	}
	if !options[0].Selected || options[0].Tag != domain.PruneTagMerged {
		t.Errorf("safe candidate = %+v, want it checked and tagged merged", options[0])
	}
	if options[1].Selected || options[1].Tag != domain.PruneTagDirty || options[1].Tone != domain.ToneDanger {
		t.Errorf("unsafe candidate = %+v, want it unchecked and tagged dirty in danger", options[1])
	}
}

// With nobody to uncheck anything, every match is kept — that is what the reason
// filters already selected. It is a decision with a safe default, not a required
// selection, so it never refuses the run.
func TestSelectionResolvesToEveryMatch(t *testing.T) {
	f := flowWith(domain.PrunePlan{Selected: []domain.PruneCandidate{
		candidate("a-wt", domain.PruneReasonPRMerged, "", "main"),
		candidate("b-wt", domain.PruneReasonGone, "", "main"),
	}}, Request{})

	answer, err := f.selectionStep().Resolve(flow.Answers{})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if strings.Join(answer.Values, ",") != "a-wt,b-wt" {
		t.Errorf("values = %v, want every match", answer.Values)
	}
}

// The reparent step is the one prune may not ask, and --reparent-children is the
// preset that answers it without asking.
func TestReparentIsPresetByTheFlag(t *testing.T) {
	plan := domain.PrunePlan{
		Selected:  []domain.PruneCandidate{candidate("parent-wt", domain.PruneReasonPRMerged, "", "main")},
		Reparents: []domain.ReparentResult{{Branch: "child-wt", OldParent: "parent-wt", NewParent: "main"}},
	}
	f := flowWith(plan, Request{ReparentChildren: true})
	prompter := &flowtest.ScriptedPrompter{
		Sets:    map[string][]string{KeySelection: {"parent-wt"}},
		Answers: map[string]string{KeyConfirm: confirmYes},
	}

	answers := ask(t, f, prompter)

	if prompter.AskedKeys() != KeySelection+","+KeyConfirm {
		t.Errorf("asked %q, want the reparent step answered by the flag", prompter.AskedKeys())
	}
	if answers.Value(KeyReparent) != orphans.Reparent {
		t.Errorf("reparent = %q, want the preset read back", answers.Value(KeyReparent))
	}
}

// A run with no surviving children skips the question rather than asking one
// with an empty proposal.
func TestReparentIsSkippedWithoutChildren(t *testing.T) {
	f := flowWith(domain.PrunePlan{
		Selected: []domain.PruneCandidate{candidate("merged-wt", domain.PruneReasonPRMerged, "", "main")},
	}, Request{})
	prompter := &flowtest.ScriptedPrompter{
		Sets:    map[string][]string{KeySelection: {"merged-wt"}},
		Answers: map[string]string{KeyConfirm: confirmYes},
	}

	ask(t, f, prompter)

	if prompter.AskedKeys() != KeySelection+","+KeyConfirm {
		t.Errorf("asked %q, want the reparent step skipped", prompter.AskedKeys())
	}
}

// The recap must state the fate of the children whichever way it was decided —
// a flag answering the step must not make the line disappear.
func TestRecapStatesTheReparentDecisionFromEitherSource(t *testing.T) {
	plan := domain.PrunePlan{
		Selected:  []domain.PruneCandidate{candidate("parent-wt", domain.PruneReasonPRMerged, "", "main")},
		Reparents: []domain.ReparentResult{{Branch: "child-wt", OldParent: "parent-wt", NewParent: "main"}},
	}

	tests := []struct {
		name    string
		request Request
		answer  string
		want    string
	}{
		{"answered in the wizard", Request{}, orphans.Reparent, "reparent 1 child"},
		{"declined in the wizard", Request{}, orphans.Orphan, "leave 1 child"},
		{"answered by the flag", Request{ReparentChildren: true}, "", "reparent 1 child"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := flowWith(plan, tt.request)
			prompter := &flowtest.ScriptedPrompter{
				Sets:    map[string][]string{KeySelection: {"parent-wt"}},
				Answers: map[string]string{KeyReparent: tt.answer, KeyConfirm: confirmYes},
			}

			ask(t, f, prompter)

			recap := prompter.Content[KeyConfirm].Description
			if !strings.Contains(recap, "parent-wt") {
				t.Errorf("recap must name what is removed:\n%s", recap)
			}
			if !strings.Contains(strings.ToLower(recap), tt.want) {
				t.Errorf("recap = %q, want it to say %q", recap, tt.want)
			}
		})
	}
}

// The force option is offered only while a refusal is actually in the way, and
// each one is named so a surface can lift them one at a time.
func TestConfirmOffersForceOnlyForACheckedUnsafeCandidate(t *testing.T) {
	f := flowWith(domain.PrunePlan{Selected: []domain.PruneCandidate{
		candidate("merged-wt", domain.PruneReasonPRMerged, "", "main"),
		candidate("dirty-wt", domain.PruneReasonGone, domain.PruneSkipDirty, "main"),
	}}, Request{})

	safe := f.confirmOptions([]string{"merged-wt"})
	if len(safe) != 1 || safe[0].Value != confirmYes {
		t.Errorf("options = %+v, want confirm alone when nothing unsafe is checked", safe)
	}

	unsafe := f.confirmOptions([]string{"merged-wt", "dirty-wt"})
	if len(unsafe) != 3 || unsafe[2].Value != confirmForce || !unsafe[2].Danger {
		t.Errorf("options = %+v, want a danger force option", unsafe)
	}

	blockers := f.blockers([]string{"merged-wt", "dirty-wt"})
	if len(blockers) != 1 || blockers[0].Key != "dirty-wt" {
		t.Errorf("blockers = %+v, want the dirty worktree named on its own", blockers)
	}
}

// prune holds the whole surface and names no target: with several worktrees to
// remove there is no single one to lock, and nothing else may run anyway.
func TestOperationBlocksWithoutATarget(t *testing.T) {
	operation := Operation()
	if operation.Kind != domain.OpKindPrune || operation.Mode != flow.ModeBlocking || operation.TargetKey != "" {
		t.Errorf("operation = %+v, want a blocking prune with no target", operation)
	}
}

// --force is the safety axis and it is not re-asked: once it lifted the refusals
// for classification, confirming plainly still removes the unsafe worktrees the
// user checked. This is a change from the pre-flow prune, which read force from
// the picker's answer alone and so dropped them again — it aligns prune with
// clean and with the two-axis model in CLAUDE.md. Pinned because it is the one
// place this migration is deliberately more destructive than what it replaced.
func TestForceSurvivesAPlainConfirmation(t *testing.T) {
	plan := domain.PrunePlan{Selected: []domain.PruneCandidate{
		candidate("merged-wt", domain.PruneReasonPRMerged, "", "main"),
		candidate("dirty-wt", domain.PruneReasonGone, domain.PruneSkipDirty, "main"),
	}}

	tests := []struct {
		name    string
		request Request
		confirm string
		want    string
	}{
		{"--force plus a plain confirmation", Request{Force: true}, confirmYes, "dirty-wt,merged-wt"},
		{"the recap's own force option", Request{}, confirmForce, "dirty-wt,merged-wt"},
		{"neither: the refusal stands", Request{}, confirmYes, "merged-wt"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := flowWith(plan, tt.request)
			prompter := &flowtest.ScriptedPrompter{
				Sets:    map[string][]string{KeySelection: {"merged-wt", "dirty-wt"}},
				Answers: map[string]string{KeyConfirm: tt.confirm},
			}
			answers := ask(t, f, prompter)

			final := rules.FinalizePrunePlan(rules.FinalizePrunePlanParams{
				Plan:       f.plan,
				Chosen:     answers.Values(KeySelection),
				BaseBranch: f.request.BaseBranch,
				Force:      f.request.Force || answers.Value(KeyConfirm) == confirmForce,
			})

			kept := make([]string, 0, len(final.Selected))
			for _, candidate := range final.Selected {
				kept = append(kept, candidate.Branch)
			}
			sort.Strings(kept)
			if strings.Join(kept, ",") != tt.want {
				t.Errorf("removed = %v, want %s", kept, tt.want)
			}
		})
	}
}

func TestDropDataAnswersTheDataStep(t *testing.T) {
	f := flowWith(domain.PrunePlan{}, Request{DropData: true})
	if got := f.session().Presets.Value(KeyData); got != owed.DataStart {
		t.Errorf("preset = %q, want %q", got, owed.DataStart)
	}
	f = flowWith(domain.PrunePlan{}, Request{})
	if got := f.session().Presets.Value(KeyData); got != "" {
		t.Errorf("preset = %q, want the step left to be answered", got)
	}
}
