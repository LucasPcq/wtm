package create_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/app/create"
	"github.com/LucasPcq/wtm/next/internal/wtm"
	"github.com/LucasPcq/wtm/next/internal/wtm/project"

	"github.com/stretchr/testify/assert"
)

func TestTheAnswersAskedToBeRememberedAreKeptOnceABranchIsCreated(t *testing.T) {
	w := newWorld(t, seed{Run: jobsToml, Behind: true})

	result, _ := w.apply(t, create.Request{Branches: []string{branchName}, EnvFrom: project.EnvExample, Isolation: project.Verbatim, FastForward: new(false), Remember: []string{"env_from", "isolation", "fast_forward"}})

	assert.Empty(t, result.Warnings)
	config := readFile(t, filepath.Join(w.stateDir, "config.toml"))
	assert.Contains(t, config, `env_strategy = "example"`)
	assert.Contains(t, config, `isolation = "verbatim"`)
	assert.Contains(t, config, `source_update = "keep"`)
}

func TestAnIsolationNotAskedIsNotRemembered(t *testing.T) {
	w := newWorld(t, seed{})

	w.apply(t, create.Request{Branches: []string{branchName}, Remember: []string{"isolation", "env_from"}})

	config := readFile(t, filepath.Join(w.stateDir, "config.toml"))
	assert.Contains(t, config, `env_strategy = "main"`)
	assert.NotContains(t, config, "isolation")
}

func TestAFastForwardNotAskedNeverOverwritesTheOneRemembered(t *testing.T) {
	w := newWorld(t, seed{Config: projectToml + "\n[wizard.remembered]\nsource_update = \"ff\"\n"})

	w.apply(t, create.Request{Branches: []string{branchName}, FastForward: new(false), Remember: []string{"fast_forward"}})

	assert.Contains(t, readFile(t, filepath.Join(w.stateDir, "config.toml")), `source_update = "ff"`)
}

func TestNothingIsRememberedWhenNoBranchWasCreated(t *testing.T) {
	w := newWorld(t, seed{Linked: []string{"linked"}})

	w.apply(t, create.Request{Branches: []string{"linked"}, IfNotExists: true, EnvFrom: project.EnvExample, Remember: []string{"env_from"}})

	assert.NotContains(t, readFile(t, filepath.Join(w.stateDir, "config.toml")), "env_strategy")
}

func TestAnAnswerThatCannotBeRememberedIsAWarningOfTheRun(t *testing.T) {
	w := newWorld(t, seed{})
	w.deps.Remember = func(context.Context, map[string]string) (wtm.Written, error) {
		return wtm.Written{}, errors.New("read-only file system")
	}

	result, _ := w.apply(t, create.Request{Branches: []string{branchName}, Remember: []string{"env_from"}})

	assert.Equal(t, []string{"create.remember_failed"}, codes(result.Warnings))
	assert.Equal(t, "could not remember the answer: read-only file system", result.Warnings[0].Message)
	assert.Equal(t, wtm.Params{"cause": "read-only file system"}, result.Warnings[0].Params)
}

func TestARememberedAnswerWrittenWithoutItsLockSaysSo(t *testing.T) {
	w := newWorld(t, seed{})
	w.deps.Remember = func(context.Context, map[string]string) (wtm.Written, error) {
		return wtm.Written{Changed: true, Unlocked: true}, nil
	}

	result, _ := w.apply(t, create.Request{Branches: []string{branchName}, Remember: []string{"env_from"}})

	assert.Equal(t, []string{"locks.unavailable"}, codes(result.Warnings))
}
