package cli

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/service/provision"

	"github.com/LucasPcq/wtm/next/internal/command/extract"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTheWizardDrawsItsKeyScreens(t *testing.T) {
	d := open(t, sessionParams{App: newWorld(seed{}).usecase()})
	d.looksLike("source")

	d.Keys("enter")
	d.looksLike("files")

	d.Keys("space", "down", "space", "enter")
	d.looksLike("target")

	d.Keys("down", "down", "enter", "enter")
	d.looksLike("recap")
}

func TestTheRequestIsWhatWasAnswered(t *testing.T) {
	d := open(t, sessionParams{App: newWorld(seed{}).usecase()})

	d.Keys("enter", "space", "down", "space", "enter", "down", "down", "enter", "down", "enter", "enter")

	require.Equal(t, form.Finished, d.Wizard.Ending())
	assert.Equal(t, extract.Request{Source: source, Files: []string{"apps/api/a.txt", "apps/api/new.txt"}, To: other, Keep: new(true)}, d.s.Request())
	assert.NotEmpty(t, d.s.Preview().Fingerprint)
}

func TestFlagsSettleTheirStepsInTheTrail(t *testing.T) {
	d := open(t, sessionParams{App: newWorld(seed{}).usecase(), Request: settled()})

	assert.Equal(t, stepMode, d.Wizard.Current())
	screen := d.Screen()
	assert.Contains(t, screen, "✓ Source worktree: feat/login\n")
	assert.Contains(t, screen, "✓ Files: 2 files · --files")
	assert.Contains(t, screen, "✓ Target worktree: feat/other · --to")
	assert.Contains(t, screen, "· Branch name — the target worktree exists: nothing is created")
	assert.Contains(t, screen, "› Move — remove the files from feat/login")
}

func TestANewTargetGivenByItsFlagSettlesTheTargetAndTheBranch(t *testing.T) {
	req := settled()
	req.To = split
	d := open(t, sessionParams{App: newWorld(seed{}).usecase(), Request: req})

	assert.Equal(t, stepFrom, d.Wizard.Current())
	screen := d.Screen()
	assert.Contains(t, screen, "✓ Target worktree: new worktree · --to")
	assert.Contains(t, screen, "✓ Branch name: feat/split · --to")
}

func TestTheFilesStepRefusesAnEmptySelection(t *testing.T) {
	d := open(t, sessionParams{App: newWorld(seed{}).usecase(), Request: extract.Request{Source: source}})

	d.Keys("enter")

	assert.Equal(t, stepFiles, d.Wizard.Current())
	assert.Contains(t, d.Screen(), "✗ Select at least one file")
}

func TestASourceWithNothingToExtractSaysSo(t *testing.T) {
	d := open(t, sessionParams{App: newWorld(seed{Clean: true}).usecase(), Request: extract.Request{Source: source}})

	assert.Equal(t, stepFiles, d.Wizard.Current())
	assert.Contains(t, d.Screen(), "No changes to extract in feat/login — press esc to pick another worktree.")
}

func TestTheCreationStepsAreAskedOnlyForANewTarget(t *testing.T) {
	req := extract.Request{Source: source, Files: []string{"old.txt"}}

	existing := open(t, sessionParams{App: newWorld(seed{}).usecase(), Request: req})
	existing.Keys("down", "down", "enter")
	assert.Equal(t, stepMode, existing.Wizard.Current())

	created := open(t, sessionParams{App: newWorld(seed{}).usecase(), Request: req})
	created.Keys("enter")
	assert.Equal(t, stepBranch, created.Wizard.Current())
	assert.Contains(t, created.Screen(), "✓ Target worktree: new worktree")

	created.Keys("bad..name", "enter")
	assert.Equal(t, stepBranch, created.Wizard.Current())
	assert.Contains(t, created.Screen(), "is not a valid branch name")
}

func TestABranchCreatedIsAskedItsSourceThenTheMode(t *testing.T) {
	d := open(t, sessionParams{App: newWorld(seed{}).usecase(), Request: extract.Request{Source: source, Files: []string{"old.txt"}}})

	d.Keys("enter", "feat/split", "enter")
	assert.Equal(t, stepFrom, d.Wizard.Current())
	assert.Contains(t, d.Screen(), "✓ Branch name: feat/split")
	assert.Contains(t, d.Screen(), "› main")

	d.Keys("enter")
	assert.Equal(t, stepEnvFrom, d.Wizard.Current())

	d.Keys("enter")
	assert.Equal(t, stepMode, d.Wizard.Current())
	assert.Contains(t, d.Screen(), "✓ Env strategy: config default\n")
	assert.Contains(t, d.Screen(), "· Isolation — run.toml declares nothing a worktree isolates")

	d.Keys("enter")
	assert.Contains(t, d.Screen(), "Yes, create & extract")
}

func TestBackFromACreationStepReturnsToTheTarget(t *testing.T) {
	d := open(t, sessionParams{App: newWorld(seed{}).usecase(), Request: extract.Request{Source: source, Files: []string{"old.txt"}}})
	d.Keys("enter")
	require.Equal(t, stepBranch, d.Wizard.Current())

	d.Keys("esc")

	assert.Equal(t, stepTarget, d.Wizard.Current())
	d.Keys("down", "down", "enter")
	assert.Equal(t, stepMode, d.Wizard.Current())
	assert.Equal(t, other, d.s.Request().To)
}

func TestTheConflictQuestionIsAskedOnlyWhenThePlanConflicts(t *testing.T) {
	clean := open(t, sessionParams{App: newWorld(seed{}).usecase(), Request: settled()})
	clean.Keys("enter")
	assert.Equal(t, stepRecap, clean.Wizard.Current())

	conflicts := map[string]worktree.ConflictKind{"apps/api/a.txt": worktree.ConflictModified}
	d := open(t, sessionParams{App: newWorld(seed{Conflicts: conflicts}).usecase(), Request: settled()})
	d.Keys("enter")

	assert.Equal(t, stepOnConflict, d.Wizard.Current())
	screen := d.Screen()
	assert.Contains(t, screen, `apps/api/a.txt was also changed in "feat/other".`)
	assert.Contains(t, screen, `then discard the same files in "feat/login"`)

	d.Keys("down", "enter")
	assert.Equal(t, extract.OnConflictResolve, d.s.Request().OnConflict)
	assert.Contains(t, d.Screen(), "✓ On conflict: resolve")
}

func TestEachWayOutCancels(t *testing.T) {
	cases := map[string]struct {
		Request extract.Request
		Keys    []string
	}{
		"esc on the first question": {Keys: []string{"esc"}},
		"no, cancel":                {Request: settled(), Keys: []string{"enter", "down", "enter"}},
		"ctrl+c":                    {Keys: []string{"ctrl+c"}},
	}
	for name, c := range cases {
		d := open(t, sessionParams{App: newWorld(seed{}).usecase(), Request: c.Request})

		d.Keys(c.Keys...)

		assert.Equal(t, form.Cancelled, d.Wizard.Ending(), name)
	}
}

func TestTheWizardDrawsEachStepOfANewTarget(t *testing.T) {
	d := open(t, sessionParams{App: newWorld(seed{Behind: true, Isolates: true}).usecase(), Request: extract.Request{Source: source, Files: []string{"old.txt"}}})

	d.Keys("enter")
	d.looksLike("branch")

	d.Keys("feat/split", "enter")
	d.looksLike("from")

	d.Keys("enter")
	d.looksLike("env_from")

	d.Keys("enter")
	d.looksLike("isolation")

	d.Keys("enter")
	d.looksLike("ff")

	d.Keys("enter")
	d.looksLike("mode")
}

func TestTheWizardDrawsTheConflictStep(t *testing.T) {
	conflicts := map[string]worktree.ConflictKind{"apps/api/a.txt": worktree.ConflictModified}
	d := open(t, sessionParams{App: newWorld(seed{Conflicts: conflicts}).usecase(), Request: settled()})

	d.Keys("enter")
	d.looksLike("on_conflict")
}

func TestABranchAWorktreeHoldsIsRefusedAsANewTarget(t *testing.T) {
	d := open(t, sessionParams{App: newWorld(seed{}).usecase(), Request: extract.Request{Source: source, Files: []string{"old.txt"}}})

	d.Keys("enter", other, "enter")

	assert.Equal(t, stepBranch, d.Wizard.Current())
	assert.Contains(t, d.Screen(), "✗ feat/other already has a worktree — pick it as the target worktree")
}

func TestANewTargetStartsOnARememberedConfigDefaultTicked(t *testing.T) {
	remembered := map[string]string{project.RememberEnvStrategy: project.RememberEnvConfig}
	d := open(t, sessionParams{App: newWorld(seed{Remembered: remembered}).usecase(), Request: extract.Request{Source: source, Files: []string{"old.txt"}, To: split, Given: provision.Given{Ask: true}}})
	d.Keys("enter")

	require.Equal(t, stepEnvFrom, d.Wizard.Current())
	assert.Contains(t, d.Screen(), "▌› Use config default (example)")
	assert.Contains(t, d.Screen(), "[x] Always use this answer in this repo")

	d.Keys("down", "down", "enter")
	assert.Equal(t, project.EnvMain, d.s.Request().EnvFrom)
}
