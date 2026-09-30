package newwt

import (
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/tui/components"
)

func TestCreateStepsGating(t *testing.T) {
	params := WizardParams{
		IncludeBranch: true,
		SourceUpdate:  func(SourceUpdateParams) SourceUpdatePrompt { return SourceUpdatePrompt{} },
	}

	// Disabled → every embedded create step auto-skips.
	off := CreateSteps(params, func([]components.Step) bool { return false })
	if len(off.Steps) == 0 {
		t.Fatal("expected create steps")
	}
	wiz := components.NewWizard(off.Steps)
	for _, s := range off.Steps {
		if s.AutoSkip == nil || !s.AutoSkip(wiz) {
			t.Errorf("step %q should auto-skip when the gate is disabled", s.Name)
		}
	}

	// Enabled → the (otherwise ungated) branch step must not skip.
	on := CreateSteps(params, func([]components.Step) bool { return true })
	onWiz := components.NewWizard(on.Steps)
	for _, s := range on.Steps {
		if s.Name == stepBranchName {
			if s.AutoSkip == nil || s.AutoSkip(onWiz) {
				t.Errorf("branch step should not skip when the gate is enabled")
			}
		}
	}

	// No gate → the branch step carries no AutoSkip at all.
	plain := CreateSteps(params, nil)
	for _, s := range plain.Steps {
		if s.Name == stepBranchName && s.AutoSkip != nil {
			t.Errorf("ungated branch step should have no AutoSkip")
		}
	}
}

func TestCreateStepsSourceRefreshWiring(t *testing.T) {
	// A picked source (Source == "") wires the background branch refresh.
	picked := CreateSteps(WizardParams{IncludeBranch: true}, nil)
	if picked.InitCmd == nil || picked.OnMsg == nil {
		t.Error("expected branch-refresh wiring when the source is picked")
	}
	// A fixed source (--from) needs no picker, hence no refresh.
	fixed := CreateSteps(WizardParams{IncludeBranch: true, Source: "main"}, nil)
	if fixed.InitCmd != nil || fixed.OnMsg != nil {
		t.Error("expected no branch-refresh wiring when the source is fixed")
	}
}

// extract embeds this sub-flow, so the isolation question has to read the same
// there as it does in `wtm create` — same step, same prose, same recap line.
func TestIsolationStepMirrorsTheCreateFlow(t *testing.T) {
	subflow := CreateSteps(WizardParams{IsolationApplies: true}, nil)

	var found bool
	for _, step := range subflow.Steps {
		if step.Name == domain.IsolationStepName {
			found = true
		}
	}
	if !found {
		t.Fatal("the sub-flow declares no isolation step")
	}

	if steps := CreateSteps(WizardParams{}, nil).Steps; len(steps) != len(subflow.Steps)-1 {
		t.Errorf("a project with nothing to isolate must not be asked")
	}
	if steps := CreateSteps(WizardParams{IsolationApplies: true, IsolationOverride: domain.IsolationVerbatim}, nil).Steps; len(steps) != len(subflow.Steps)-1 {
		t.Errorf("--isolation answers the step, so it must not be asked")
	}
}

// A step --isolation answered still takes its recap line, and one never asked
// falls back to the project's default.
func TestIsolationRecapLine(t *testing.T) {
	if _, shown := IsolationRecapLine(nil, WizardParams{}); shown {
		t.Error("a project with nothing to isolate must not take a recap line")
	}
	line, shown := IsolationRecapLine(nil, WizardParams{IsolationApplies: true, IsolationOverride: domain.IsolationVerbatim})
	if !shown || !strings.Contains(line, domain.IsolationSummaryVerbatim) {
		t.Errorf("recap line = %q (shown=%v), want the flag's answer named", line, shown)
	}
	if got := resolveIsolation(nil, WizardParams{IsolationDefault: domain.IsolationVerbatim}); got != domain.IsolationVerbatim {
		t.Errorf("unasked isolation = %q, want the project's default", got)
	}
}

// Aligned on create: a flag keeps its line even where run.toml declares
// nothing, and the line is labelled after the step that asks it.
func TestIsolationRecapLineFollowsTheCreateFlow(t *testing.T) {
	line, shown := IsolationRecapLine(nil, WizardParams{IsolationOverride: domain.IsolationVerbatim})
	if !shown || !strings.HasPrefix(line, "Isolation: ") {
		t.Errorf("recap line = %q (shown=%v), want the flag's answer under Isolation:", line, shown)
	}
}
