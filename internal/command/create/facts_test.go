package create_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LucasPcq/wtm/internal/command/create"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra/git"
	"github.com/LucasPcq/wtm/internal/kernel"
)

func TestARepositoryWithoutConfigIsAPrecondition(t *testing.T) {
	w := newWorld(t, seed{})
	require.NoError(t, os.Remove(filepath.Join(w.stateDir, domain.ConfigFileName)))

	_, err := create.Command(w.ports).Observe(t.Context())

	var failure kernel.Error
	require.ErrorAs(t, err, &failure)
	assert.Equal(t, kernel.KindPrecondition, failure.Kind())
	assert.Equal(t, create.CodeConfigNotFound, failure.Base().Code)
}

func TestTheFactsCountOnlyTheBranchesThatDifferFromOrigin(t *testing.T) {
	w := newWorld(t, seed{Behind: true})

	facts := w.facts(t)

	assert.Equal(t, map[string]git.AheadBehind{"main": {Behind: 1}}, facts.Divergence)
}

func TestTheFactsNameEachFolderAlreadyUnderTheBasePath(t *testing.T) {
	w := newWorld(t, seed{})
	require.NoError(t, os.MkdirAll(filepath.Join(w.trees, "plain"), 0o755))

	facts := w.facts(t)

	assert.Equal(t, map[string]git.Location{"plain": {Path: filepath.Join(w.trees, "plain"), Exists: true}}, facts.Taken)
}

func TestARunConfigThatDoesNotLoadIsAFactNotAFailure(t *testing.T) {
	w := newWorld(t, seed{Run: "[[job]]\nname = 3\n"})

	facts := w.facts(t)

	assert.NotEmpty(t, facts.RunProblem)
	assert.Equal(t, domain.RunConfig{}, facts.Run)
}

func TestForgettingDropsTheRememberedAnswersOnly(t *testing.T) {
	w := newWorld(t, seed{Config: project + "\n[wizard.remembered]\nisolation = \"verbatim\"\n"})
	facts := w.facts(t)

	forgotten := create.Command(w.ports).Forget(facts)

	assert.Empty(t, forgotten.Config.Wizard.Remembered)
	assert.Equal(t, facts.Config.Worktrees, forgotten.Config.Worktrees)
}
