package newwt

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/tui/components"
)

// These tests characterize the declarative create wizard — the step set built
// from the flags, the answers read back out of it, and the recap — so a refactor
// of the flow cannot silently change what the wizard asks or reports.

// ffOffer is a SourceUpdate decider offering a fast-forward on branch.
func ffOffer(branch string) func(SourceUpdateParams) SourceUpdatePrompt {
	return func(SourceUpdateParams) SourceUpdatePrompt {
		return SourceUpdatePrompt{
			Branch: branch,
			Show:   true,
			Params: components.NewConfirmParams{Description: "behind origin"},
		}
	}
}

// divergedOffer is a SourceUpdate decider reporting a diverged branch: a warning
// for the recap, never an actionable wizard step.
func divergedOffer(branch string) func(SourceUpdateParams) SourceUpdatePrompt {
	return func(SourceUpdateParams) SourceUpdatePrompt {
		return SourceUpdatePrompt{
			Branch:         branch,
			Show:           true,
			Params:         components.NewConfirmParams{Warning: domain.SourceDivergedWarning},
			AbortOnDecline: true,
			SkipReason:     "source diverged from origin — see recap",
		}
	}
}

// upToDateOffer is a SourceUpdate decider with nothing to reconcile.
func upToDateOffer() func(SourceUpdateParams) SourceUpdatePrompt {
	return func(SourceUpdateParams) SourceUpdatePrompt {
		return SourceUpdatePrompt{SkipReason: "source already up to date"}
	}
}

func stepNames(steps []components.Step) []string {
	names := make([]string, 0, len(steps))
	for _, s := range steps {
		names = append(names, s.Name)
	}
	return names
}

// TestCreateStepsCompositionFollowsFlags locks which questions the wizard asks
// for each flag combination: a flag fixes a value and removes its step, and the
// conditional source-update step exists only when a fast-forward is offered.
func TestCreateStepsCompositionFollowsFlags(t *testing.T) {
	tests := []struct {
		name   string
		params WizardParams
		want   []string
	}{
		{
			name:   "bare create asks everything",
			params: WizardParams{IncludeBranch: true, IncludeEnv: true, SourceUpdate: ffOffer("main")},
			want:   []string{stepBranchName, stepSourceName, stepEnvName, stepSourceUpd},
		},
		{
			name:   "--from fixes the source",
			params: WizardParams{IncludeBranch: true, IncludeEnv: true, Source: "main", SourceUpdate: ffOffer("main")},
			want:   []string{stepBranchName, stepEnvName, stepSourceUpd},
		},
		{
			name:   "--env-from fixes the env strategy",
			params: WizardParams{IncludeBranch: true, Source: "main", EnvOverride: "example", SourceUpdate: ffOffer("main")},
			want:   []string{stepBranchName, stepSourceUpd},
		},
		{
			name:   "branch arg with --from and --env-from keeps only the fast-forward",
			params: WizardParams{BranchName: "feat/x", Source: "main", EnvOverride: "example", SourceUpdate: ffOffer("main")},
			want:   []string{stepSourceUpd},
		},
		{
			name:   "an up-to-date source leaves no step at all",
			params: WizardParams{BranchName: "feat/x", Source: "main", EnvOverride: "example", SourceUpdate: upToDateOffer()},
			want:   nil,
		},
		{
			name:   "a diverged source is a recap warning, not a step",
			params: WizardParams{BranchName: "feat/x", Source: "main", EnvOverride: "example", SourceUpdate: divergedOffer("main")},
			want:   nil,
		},
		{
			name:   "no SourceUpdate decider means no fast-forward step",
			params: WizardParams{IncludeBranch: true, IncludeEnv: true},
			want:   []string{stepBranchName, stepSourceName, stepEnvName},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stepNames(CreateSteps(tt.params, nil).Steps)
			if strings.Join(got, ",") != strings.Join(tt.want, ",") {
				t.Errorf("steps = %v, want %v", got, tt.want)
			}
		})
	}
}

// runeKeys types s into the current step, one key at a time.
func runeKeys(m components.WizardModel, s string) components.WizardModel {
	for _, r := range s {
		m = updateWiz(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	return m
}

func updateWiz(m components.WizardModel, msg tea.Msg) components.WizardModel {
	model, _ := m.Update(msg)
	updated, ok := model.(components.WizardModel)
	if !ok {
		panic("wizard Update returned a foreign model")
	}
	return updated
}

// TestCreateWizardReadsTypedBranchAndAcceptedFastForward drives the real steps
// headlessly: the typed branch name and the accepted fast-forward must come back
// out of ReadCreateResult, with the flag-fixed source and env preserved.
func TestCreateWizardReadsTypedBranchAndAcceptedFastForward(t *testing.T) {
	params := WizardParams{
		IncludeBranch: true,
		Source:        "main",
		EnvOverride:   "example",
		SourceUpdate:  ffOffer("main"),
	}

	m := components.NewWizard(CreateSteps(params, nil).Steps)
	m.Init()
	m = runeKeys(m, "feat/x")
	m = updateWiz(m, tea.KeyMsg{Type: tea.KeyEnter}) // branch name → source update
	m = updateWiz(m, tea.KeyMsg{Type: tea.KeyEnter}) // first option: fast-forward

	got := ReadCreateResult(m.Steps(), params)
	if got.BranchName != "feat/x" {
		t.Errorf("BranchName = %q, want %q", got.BranchName, "feat/x")
	}
	if got.FromBranch != "main" {
		t.Errorf("FromBranch = %q, want the --from value %q", got.FromBranch, "main")
	}
	if got.EnvFromOverride != "example" {
		t.Errorf("EnvFromOverride = %q, want the --env-from value %q", got.EnvFromOverride, "example")
	}
	if got.FastForwardBranch != "main" {
		t.Errorf("FastForwardBranch = %q, want %q", got.FastForwardBranch, "main")
	}
}

// TestCreateWizardKeepAsIsLeavesNoFastForward is the other half of the choice:
// declining the offer must not schedule any branch movement.
func TestCreateWizardKeepAsIsLeavesNoFastForward(t *testing.T) {
	params := WizardParams{
		IncludeBranch: true,
		Source:        "main",
		EnvOverride:   "example",
		SourceUpdate:  ffOffer("main"),
	}

	m := components.NewWizard(CreateSteps(params, nil).Steps)
	m.Init()
	m = runeKeys(m, "feat/x")
	m = updateWiz(m, tea.KeyMsg{Type: tea.KeyEnter}) // branch name → source update
	m = updateWiz(m, tea.KeyMsg{Type: tea.KeyDown})  // skip the separator → "Keep it as-is"
	m = updateWiz(m, tea.KeyMsg{Type: tea.KeyEnter})

	if got := ReadCreateResult(m.Steps(), params).FastForwardBranch; got != "" {
		t.Errorf("FastForwardBranch = %q, want empty after keeping the branch as-is", got)
	}
}

// TestCreateWizardRejectsEmptyBranchName keeps the branch step's validation: an
// empty name must not advance the wizard.
func TestCreateWizardRejectsEmptyBranchName(t *testing.T) {
	params := WizardParams{IncludeBranch: true, Source: "main", EnvOverride: "example"}

	m := components.NewWizard(CreateSteps(params, nil).Steps)
	m.Init()
	m = updateWiz(m, tea.KeyMsg{Type: tea.KeyEnter})

	if m.Done() {
		t.Fatal("an empty branch name must not complete the wizard")
	}
	if got := ReadCreateResult(m.Steps(), params).BranchName; got != "" {
		t.Errorf("BranchName = %q, want empty", got)
	}
}

// TestReadCreateResultFallsBackToFlags covers the no-step path: every value was
// fixed by a flag or the positional arg, so the result comes from the params.
func TestReadCreateResultFallsBackToFlags(t *testing.T) {
	params := WizardParams{
		BranchName:   "feat/x",
		Source:       "main",
		EnvOverride:  "parent",
		SourceUpdate: upToDateOffer(),
		Target:       existingTarget("feat/x"),
	}

	got := ReadCreateResult(CreateSteps(params, nil).Steps, params)
	if got.FromBranch != "main" || got.EnvFromOverride != "parent" {
		t.Errorf("result = %+v, want the --from/--env-from values", got)
	}
	if got.FastForwardBranch != "" {
		t.Errorf("FastForwardBranch = %q, want empty when nothing was offered", got.FastForwardBranch)
	}
	if !got.Reused {
		t.Error("Reused should report the existing local branch")
	}
	// BranchName is read from the branch step only; the host keeps the arg value.
	if got.BranchName != "" {
		t.Errorf("BranchName = %q, want empty without a branch step", got.BranchName)
	}
}

// existingTarget classifies branch as an already-existing local branch.
func existingTarget(branch string) func(string) domain.BranchTarget {
	return func(b string) domain.BranchTarget {
		if b == branch {
			return domain.BranchTarget{State: domain.BranchTargetExisting}
		}
		return domain.BranchTarget{}
	}
}

// TestCreateWarningsSurfaceBothDeciders: a diverged source and the env fallback
// are the two ⚠ lines the recap must carry — they are the only warning of a
// consequence the user cannot see anywhere else.
func TestCreateWarningsSurfaceBothDeciders(t *testing.T) {
	params := WizardParams{
		BranchName:   "feat/x",
		Source:       "main",
		EnvOverride:  "parent",
		SourceUpdate: divergedOffer("main"),
		EnvFallback: func(string, string) (bool, components.NewConfirmParams) {
			return true, components.NewConfirmParams{Warning: "env falls back to main"}
		},
	}

	warnings := CreateWarnings(nil, params)
	if len(warnings) != 2 {
		t.Fatalf("warnings = %v, want the diverged-source and env-fallback lines", warnings)
	}
	for _, w := range warnings {
		if !strings.HasPrefix(w, "⚠ ") {
			t.Errorf("warning %q should be marked with ⚠", w)
		}
	}
}

// TestCreateWarningsSilentWhenNothingToWarnAbout guards the other direction: no
// noise when the source is clean and env needs no fallback.
func TestCreateWarningsSilentWhenNothingToWarnAbout(t *testing.T) {
	warnings := CreateWarnings(nil, WizardParams{
		BranchName:   "feat/x",
		Source:       "main",
		SourceUpdate: ffOffer("main"),
		EnvFallback: func(string, string) (bool, components.NewConfirmParams) {
			return false, components.NewConfirmParams{}
		},
	})
	if len(warnings) != 0 {
		t.Errorf("warnings = %v, want none", warnings)
	}
}
