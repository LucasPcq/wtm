package rules

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func monorepoScripts() []domain.PackageScript {
	return []domain.PackageScript{
		{Name: "build", Cmd: "turbo run build"},
		{Name: "build", Workspace: "apps/web", Cmd: "vite build"},
		{Name: "dev", Workspace: "apps/web", Cmd: "vite"},
	}
}

func TestApplyScriptKindsSettlesEachPackageOnItsOwn(t *testing.T) {
	// Two packages declaring "build" are two answers: settling one must leave
	// the other alone.
	got := ApplyScriptKinds(ApplyScriptKindsParams{
		Scripts: monorepoScripts(),
		Choices: []domain.JobKindChoice{
			{Name: "build", Kind: domain.JobKindService},
			{Name: "build", Workspace: "apps/web", Kind: domain.JobKindTask},
		},
	})

	if got[0].Kind != domain.JobKindService {
		t.Errorf("root build = %q, want service", got[0].Kind)
	}
	if got[1].Kind != domain.JobKindTask {
		t.Errorf("apps/web build = %q, want task — the root answer leaked into it", got[1].Kind)
	}
}

func TestApplyScriptKindsLeavesAScriptItWasNotAskedAbout(t *testing.T) {
	got := ApplyScriptKinds(ApplyScriptKindsParams{
		Scripts: monorepoScripts(),
		Choices: []domain.JobKindChoice{{Name: "build", Kind: domain.JobKindService}},
	})

	if got[2].Kind != "" {
		t.Errorf("dev = %q, want the kind its name implies, settled downstream", got[2].Kind)
	}
}

func TestApplyScriptKindsDoesNotMutateItsInput(t *testing.T) {
	scripts := monorepoScripts()
	ApplyScriptKinds(ApplyScriptKindsParams{
		Scripts: scripts,
		Choices: []domain.JobKindChoice{{Name: "build", Kind: domain.JobKindService}},
	})

	if scripts[0].Kind != "" {
		t.Errorf("the input was mutated: %q", scripts[0].Kind)
	}
}

func TestScriptKindsSkipReasonTellsWhichEmptinessItWas(t *testing.T) {
	if got := ScriptKindsSkipReason(0); got != domain.SkipReasonNoScriptChecked {
		t.Errorf("with nothing checked, reason = %q", got)
	}
	if got := ScriptKindsSkipReason(2); got != domain.SkipReasonKindsSettled {
		t.Errorf("with only dev scripts checked, reason = %q", got)
	}
}
