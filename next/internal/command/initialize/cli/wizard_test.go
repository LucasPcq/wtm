package cli

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Legacy's trail named two steps "Post-create hooks" and counted "1 files selected", and it wrote a first init with no recap.
func TestTheWizardDrawsEachStepThenTheRecap(t *testing.T) {
	d := open(t, newWorld(seed{}))
	d.looksLike("base_path")

	d.Keys("enter")
	d.looksLike("base_branch")
	d.Keys("enter")
	d.looksLike("env_strategy")
	d.Keys("enter")
	d.looksLike("env_files")
	d.Keys("enter")
	d.looksLike("on_create")
	d.Keys("down", "down", "enter")
	d.looksLike("on_clean")
	d.Keys("down", "down", "enter")
	d.looksLike("recap")
}

func TestAReInitPicksOnlyTheFilesSetUpAndKeepsTheOthersListed(t *testing.T) {
	d := open(t, newWorld(seed{Saved: setUp()}))

	d.Keys("enter", "enter", "enter")

	d.looksLike("re_init_env_files")
}

func TestAHookAddedInTheWizardIsAnswered(t *testing.T) {
	d := open(t, newWorld(seed{}))

	d.Keys("enter", "enter", "enter", "enter")
	d.Keys("down", "enter")
	d.Keys("make db")
	d.Keys("tab")
	d.Keys("apps/api")
	d.Keys("enter", "down", "down", "enter", "down", "down", "enter", "enter")

	require.Equal(t, form.Finished, d.Wizard.Ending())
	assert.Equal(t, []project.Hook{{Cmd: "pnpm install"}, {Cmd: "make db", Cwd: "apps/api"}}, d.s.Request().OnCreate)
}
