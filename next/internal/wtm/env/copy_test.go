package env

import (
	"errors"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/wtm/project"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSourcesNameWhatTheyCouldNotFind(t *testing.T) {
	present := map[string]bool{"/main/.env": true}

	example, exampleWarnings := sources(t, SourcesParams{Request: CopyRequest{Strategy: project.EnvExample, Files: []project.EnvFile{{Target: ".env", Template: ".env.example"}}, TargetDir: "/wt", MainPath: "/main"}, Exists: existsIn(present)})
	missing, missingWarnings := sources(t, SourcesParams{Request: CopyRequest{Strategy: project.EnvMain, Files: []project.EnvFile{{Target: ".env.local"}}, TargetDir: "/wt", MainPath: "/main"}, Exists: existsIn(present)})
	fallback, fallbackWarnings := sources(t, SourcesParams{Request: CopyRequest{Strategy: project.EnvParent, Files: []project.EnvFile{{Target: ".env"}, {Target: ".env.local"}}, TargetDir: "/wt", MainPath: "/main", ParentPath: "/parent"}, Exists: existsIn(present)})

	assert.Empty(t, example)
	assert.Equal(t, []CopyWarning{NoTemplate{Target: ".env"}}, exampleWarnings)
	assert.Empty(t, missing)
	assert.Equal(t, []CopyWarning{SourceMissing{Target: ".env.local", Dir: "/main"}}, missingWarnings)
	assert.Equal(t, []Source{{From: "/main/.env", To: "/wt/.env"}}, fallback)
	assert.Equal(t, []CopyWarning{
		ParentFallback{Target: ".env"},
		ParentFallback{Target: ".env.local"},
		SourceMissing{Target: ".env.local", Dir: "/main"},
	}, fallbackWarnings)
}

func TestTheMainStrategyCopiesFromTheMainCheckout(t *testing.T) {
	request := CopyRequest{Strategy: project.EnvMain, Files: []project.EnvFile{{Target: "apps/api/.env"}}, TargetDir: "/wt", MainPath: "/main"}

	found, warnings := sources(t, SourcesParams{Request: request, Exists: existsIn(map[string]bool{"/main/apps/api/.env": true})})

	assert.Equal(t, []Source{{From: "/main/apps/api/.env", To: "/wt/apps/api/.env"}}, found)
	assert.Empty(t, warnings)
}

func TestTheParentStrategyCopiesFromTheParentWhenItHoldsTheFile(t *testing.T) {
	request := CopyRequest{Strategy: project.EnvParent, Files: []project.EnvFile{{Target: ".env"}}, TargetDir: "/wt", MainPath: "/main", ParentPath: "/parent"}

	found, warnings := sources(t, SourcesParams{Request: request, Exists: existsIn(map[string]bool{"/parent/.env": true, "/main/.env": true})})

	assert.Equal(t, []Source{{From: "/parent/.env", To: "/wt/.env"}}, found)
	assert.Empty(t, warnings)
}

func TestTheParentStrategyWithoutAParentWorktreeCopiesFromMainSilently(t *testing.T) {
	request := CopyRequest{Strategy: project.EnvParent, Files: []project.EnvFile{{Target: ".env"}}, TargetDir: "/wt", MainPath: "/main"}

	found, warnings := sources(t, SourcesParams{Request: request, Exists: existsIn(map[string]bool{"/main/.env": true})})

	assert.Equal(t, []Source{{From: "/main/.env", To: "/wt/.env"}}, found)
	assert.Empty(t, warnings)
}

func TestCandidatesAreEveryPathSourcesMayLookAt(t *testing.T) {
	files := []project.EnvFile{{Target: ".env"}}

	example, err := candidates(CopyRequest{Strategy: project.EnvExample, Files: files, MainPath: "/main"})
	require.NoError(t, err)
	parent, err := candidates(CopyRequest{Strategy: project.EnvParent, Files: files, MainPath: "/main", ParentPath: "/parent"})
	require.NoError(t, err)
	main, err := candidates(CopyRequest{Strategy: project.EnvMain, Files: files, MainPath: "/main"})
	require.NoError(t, err)

	assert.Equal(t, []string{"/main/.env.example", "/main/.env.dist", "/main/.env.sample", "/main/.env.template", "/main/.env.tmpl"}, example)
	assert.Equal(t, []string{"/parent/.env", "/main/.env"}, parent)
	assert.Equal(t, []string{"/main/.env"}, main)
}

func TestAnUnknownStrategyIsRefused(t *testing.T) {
	request := CopyRequest{Strategy: "elsewhere", Files: []project.EnvFile{{Target: ".env"}}}

	_, _, err := Sources(SourcesParams{Request: request})
	assert.EqualError(t, err, "unknown env strategy: elsewhere")
	_, err = candidates(request)
	assert.EqualError(t, err, "unknown env strategy: elsewhere")
}

func TestAPathThatCannotBeCheckedStopsTheSources(t *testing.T) {
	denied := errors.New("permission denied")
	request := CopyRequest{Strategy: project.EnvMain, Files: []project.EnvFile{{Target: ".env"}}, TargetDir: "/wt", MainPath: "/main"}

	_, _, err := Sources(SourcesParams{Request: request, Exists: func(string) (bool, error) { return false, denied }})

	assert.ErrorIs(t, err, denied)
}

func TestTheTemplateIsThePinnedOneElseTheFirstConventionalSpellingPresent(t *testing.T) {
	guessed := CopyRequest{Strategy: project.EnvExample, Files: []project.EnvFile{{Target: ".env"}}, TargetDir: "/wt", MainPath: "/main"}
	pinned := CopyRequest{Strategy: project.EnvExample, Files: []project.EnvFile{{Target: ".env", Template: ".env.example"}}, TargetDir: "/wt", MainPath: "/main"}
	present := map[string]bool{"/main/.env.sample": true, "/main/.env.tmpl": true}

	found, _ := sources(t, SourcesParams{Request: guessed, Exists: existsIn(present)})
	gone, warnings := sources(t, SourcesParams{Request: pinned, Exists: existsIn(present)})

	assert.Equal(t, []Source{{From: "/main/.env.sample", To: "/wt/.env"}}, found)
	assert.Empty(t, gone, "a pinned template that is gone is not replaced by a guess")
	assert.Equal(t, []CopyWarning{NoTemplate{Target: ".env"}}, warnings)
}

func TestTheParentStrategyFallsBackToMainOnlyWithoutTheSourcesWorktree(t *testing.T) {
	assert.True(t, ParentFallsBackToMain(ParentFallbackParams{Strategy: project.EnvParent, HasFiles: true}))
	assert.False(t, ParentFallsBackToMain(ParentFallbackParams{Strategy: project.EnvParent, HasFiles: true, SourceHasWorktree: true}))
	assert.False(t, ParentFallsBackToMain(ParentFallbackParams{Strategy: project.EnvParent}))
	assert.False(t, ParentFallsBackToMain(ParentFallbackParams{Strategy: project.EnvMain, HasFiles: true}))
	assert.False(t, ParentFallsBackToMain(ParentFallbackParams{Strategy: project.EnvExample, HasFiles: true}))
}
