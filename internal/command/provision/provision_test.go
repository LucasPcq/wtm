package provision_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/LucasPcq/wtm/internal/command/provision"
	"github.com/LucasPcq/wtm/internal/domain"
)

func TestAProjectWithoutRunConfigOrHooksStopsAtTheSaga(t *testing.T) {
	w, _ := inMemory(t, seeds{})

	assert.Empty(t, phases(provision.Unit(w.params())))
}

func TestALinkedWorktreeWithPortLinksIsNumberedThenSettled(t *testing.T) {
	w, _ := inMemory(t, seeds{})

	assert.Equal(t, []string{provision.PhaseOrdinal, provision.PhasePorts}, phases(provision.Unit(portLinked(w.params()))))
}

func TestAVerbatimWorktreeIsNeitherNumberedNorSettled(t *testing.T) {
	w, _ := inMemory(t, seeds{})
	params := portLinked(w.params())
	params.Worktree.Isolation = domain.IsolationVerbatim

	assert.Empty(t, phases(provision.Unit(params)))
}

func TestARunConfigThatDoesNotLoadStillHasItsPortPassToSaySo(t *testing.T) {
	w, _ := inMemory(t, seeds{})
	params := portLinked(w.params())
	params.Project.RunErr = errors.New("run.toml: line 3")

	assert.Equal(t, []string{provision.PhasePorts}, phases(provision.Unit(params)))
}

func TestHooksAreNumberedFirstOnlyWhenTheyReadTheRunEnvironment(t *testing.T) {
	w, _ := inMemory(t, seeds{})

	assert.Equal(t, []string{string(domain.HookOnCreate)}, phases(provision.Unit(hooked(w.params(), "true"))))
	assert.Equal(t, []string{provision.PhaseOrdinal, provision.PhasePorts, string(domain.HookOnCreate)}, phases(provision.Unit(hooked(composed(w.params()), "true"))))
}
