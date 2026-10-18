package cli

import (
	"github.com/LucasPcq/wtm/next/internal/service/provision"
	"testing"

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

	d.keys("enter")
	d.looksLike("files")

	d.keys("space", "down", "space", "enter")
	d.looksLike("target")

	d.keys("down", "down", "enter", "enter")
	d.looksLike("recap")
}

func TestTheRequestIsWhatWasAnswered(t *testing.T) {
	d := open(t, sessionParams{App: newWorld(seed{}).usecase()})

	d.keys("enter", "space", "down", "space", "enter", "down", "down", "enter", "down", "enter", "enter")

	require.Equal(t, form.Finished, d.w.Ending())
	assert.Equal(t, extract.Request{Source: source, Files: []string{"apps/api/a.txt", "apps/api/new.txt"}, To: other, Keep: new(true)}, d.s.Request())
	assert.NotEmpty(t, d.s.Preview().Fingerprint)
}

func TestFlagsSettleTheirStepsInTheTrail(t *testing.T) {
	d := open(t, sessionParams{App: newWorld(seed{}).usecase(), Request: settled()})

	assert.Equal(t, stepMode, d.w.Current())
	screen := d.screen()
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

	assert.Equal(t, stepFrom, d.w.Current())
	screen := d.screen()
	assert.Contains(t, screen, "✓ Target worktree: new worktree · --to")
	assert.Contains(t, screen, "✓ Branch name: feat/split · --to")
}

func TestTheFilesStepRefusesAnEmptySelection(t *testing.T) {
	d := open(t, sessionParams{App: newWorld(seed{}).usecase(), Request: extract.Request{Source: source}})

	d.keys("enter")

	assert.Equal(t, stepFiles, d.w.Current())
	assert.Contains(t, d.screen(), "✗ Select at least one file")
}

func TestASourceWithNothingToExtractSaysSo(t *testing.T) {
	d := open(t, sessionParams{App: newWorld(seed{Clean: true}).usecase(), Request: extract.Request{Source: source}})

	assert.Equal(t, stepFiles, d.w.Current())
	assert.Contains(t, d.screen(), "No changes to extract in feat/login — press esc to pick another worktree.")
}

func TestTheCreationStepsAreAskedOnlyForANewTarget(t *testing.T) {
	req := extract.Request{Source: source, Files: []string{"old.txt"}}

	existing := open(t, sessionParams{App: newWorld(seed{}).usecase(), Request: req})
	existing.keys("down", "down", "enter")
	assert.Equal(t, stepMode, existing.w.Current())

	created := open(t, sessionParams{App: newWorld(seed{}).usecase(), Request: req})
	created.keys("enter")
	assert.Equal(t, stepBranch, created.w.Current())
	assert.Contains(t, created.screen(), "✓ Target worktree: new worktree")

	created.keys("bad..name", "enter")
	assert.Equal(t, stepBranch, created.w.Current())
	assert.Contains(t, created.screen(), "is not a valid branch name")
}

func TestABranchCreatedIsAskedItsSourceThenTheMode(t *testing.T) {
	d := open(t, sessionParams{App: newWorld(seed{}).usecase(), Request: extract.Request{Source: source, Files: []string{"old.txt"}}})

	d.keys("enter", "feat/split", "enter")
	assert.Equal(t, stepFrom, d.w.Current())
	assert.Contains(t, d.screen(), "✓ Branch name: feat/split")
	assert.Contains(t, d.screen(), "› main")

	d.keys("enter")
	assert.Equal(t, stepEnvFrom, d.w.Current())

	d.keys("enter")
	assert.Equal(t, stepMode, d.w.Current())
	assert.Contains(t, d.screen(), "✓ Env strategy: config default\n")
	assert.Contains(t, d.screen(), "· Isolation — run.toml declares nothing a worktree isolates")

	d.keys("enter")
	assert.Contains(t, d.screen(), "Yes, create & extract")
}

func TestBackFromACreationStepReturnsToTheTarget(t *testing.T) {
	d := open(t, sessionParams{App: newWorld(seed{}).usecase(), Request: extract.Request{Source: source, Files: []string{"old.txt"}}})
	d.keys("enter")
	require.Equal(t, stepBranch, d.w.Current())

	d.keys("esc")

	assert.Equal(t, stepTarget, d.w.Current())
	d.keys("down", "down", "enter")
	assert.Equal(t, stepMode, d.w.Current())
	assert.Equal(t, other, d.s.Request().To)
}

func TestTheConflictQuestionIsAskedOnlyWhenThePlanConflicts(t *testing.T) {
	clean := open(t, sessionParams{App: newWorld(seed{}).usecase(), Request: settled()})
	clean.keys("enter")
	assert.Equal(t, stepRecap, clean.w.Current())

	conflicts := map[string]worktree.ConflictKind{"apps/api/a.txt": worktree.ConflictModified}
	d := open(t, sessionParams{App: newWorld(seed{Conflicts: conflicts}).usecase(), Request: settled()})
	d.keys("enter")

	assert.Equal(t, stepOnConflict, d.w.Current())
	screen := d.screen()
	assert.Contains(t, screen, `apps/api/a.txt was also changed in "feat/other".`)
	assert.Contains(t, screen, `then discard the same files in "feat/login"`)

	d.keys("down", "enter")
	assert.Equal(t, extract.OnConflictResolve, d.s.Request().OnConflict)
	assert.Contains(t, d.screen(), "✓ On conflict: resolve")
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

		d.keys(c.Keys...)

		assert.Equal(t, form.Cancelled, d.w.Ending(), name)
	}
}

func TestTheWizardDrawsEachStepOfANewTarget(t *testing.T) {
	d := open(t, sessionParams{App: newWorld(seed{Behind: true, Isolates: true}).usecase(), Request: extract.Request{Source: source, Files: []string{"old.txt"}}})

	d.keys("enter")
	d.looksLike("branch")

	d.keys("feat/split", "enter")
	d.looksLike("from")

	d.keys("enter")
	d.looksLike("env_from")

	d.keys("enter")
	d.looksLike("isolation")

	d.keys("enter")
	d.looksLike("ff")

	d.keys("enter")
	d.looksLike("mode")
}

func TestTheWizardDrawsTheConflictStep(t *testing.T) {
	conflicts := map[string]worktree.ConflictKind{"apps/api/a.txt": worktree.ConflictModified}
	d := open(t, sessionParams{App: newWorld(seed{Conflicts: conflicts}).usecase(), Request: settled()})

	d.keys("enter")
	d.looksLike("on_conflict")
}

func TestABranchAWorktreeHoldsIsRefusedAsANewTarget(t *testing.T) {
	d := open(t, sessionParams{App: newWorld(seed{}).usecase(), Request: extract.Request{Source: source, Files: []string{"old.txt"}}})

	d.keys("enter", other, "enter")

	assert.Equal(t, stepBranch, d.w.Current())
	assert.Contains(t, d.screen(), "✗ feat/other already has a worktree — pick it as the target worktree")
}

func TestANewTargetStartsOnARememberedConfigDefaultTicked(t *testing.T) {
	remembered := map[string]string{project.RememberEnvStrategy: project.RememberEnvConfig}
	d := open(t, sessionParams{App: newWorld(seed{Remembered: remembered}).usecase(), Request: extract.Request{Source: source, Files: []string{"old.txt"}, To: split, Given: provision.Given{Ask: true}}})
	d.keys("enter")

	require.Equal(t, stepEnvFrom, d.w.Current())
	assert.Contains(t, d.screen(), "▌› Use config default (example)")
	assert.Contains(t, d.screen(), "[x] Always use this answer in this repo")

	d.keys("down", "down", "enter")
	assert.Equal(t, project.EnvMain, d.s.Request().EnvFrom)
}
