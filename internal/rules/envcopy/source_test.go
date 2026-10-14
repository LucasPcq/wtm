package envcopy_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules/envcopy"
)

func TestSourcesNameWhatTheyCouldNotFind(t *testing.T) {
	present := map[string]bool{"/main/.env": true}
	sources := func(request envcopy.Request) ([]envcopy.Source, []envcopy.Warning) {
		t.Helper()
		found, warnings, err := envcopy.Sources(envcopy.SourcesParams{Request: request, Present: present})
		require.NoError(t, err)
		return found, warnings
	}

	exampleSources, example := sources(envcopy.Request{Strategy: domain.EnvStrategyExample, Files: []domain.EnvFile{{Target: ".env", Template: ".env.example"}}, TargetDir: "/wt", MainWorktreePath: "/main"})
	missingSources, missing := sources(envcopy.Request{Strategy: domain.EnvStrategyMain, Files: []domain.EnvFile{{Target: ".env.local"}}, TargetDir: "/wt", MainWorktreePath: "/main"})
	fallbackSources, fallback := sources(envcopy.Request{Strategy: domain.EnvStrategyParent, Files: []domain.EnvFile{{Target: ".env"}, {Target: ".env.local"}}, TargetDir: "/wt", MainWorktreePath: "/main", ParentWorktreePath: "/parent"})

	assert.Empty(t, exampleSources)
	assert.Equal(t, []envcopy.Warning{envcopy.NoTemplate{Target: ".env"}}, example)
	assert.Empty(t, missingSources)
	assert.Equal(t, []envcopy.Warning{envcopy.SourceMissing{Target: ".env.local", Dir: "/main"}}, missing)
	assert.Equal(t, []envcopy.Source{{Src: "/main/.env", Dst: "/wt/.env"}}, fallbackSources)
	assert.Equal(t, []envcopy.Warning{
		envcopy.ParentFallback{Target: ".env"},
		envcopy.ParentFallback{Target: ".env.local"},
		envcopy.SourceMissing{Target: ".env.local", Dir: "/main"},
	}, fallback)
}

func TestTheParentStrategyCopiesFromTheParentWhenItHoldsTheFile(t *testing.T) {
	request := envcopy.Request{Strategy: domain.EnvStrategyParent, Files: []domain.EnvFile{{Target: ".env"}}, TargetDir: "/wt", MainWorktreePath: "/main", ParentWorktreePath: "/parent"}

	sources, warnings, err := envcopy.Sources(envcopy.SourcesParams{Request: request, Present: map[string]bool{"/parent/.env": true, "/main/.env": true}})

	require.NoError(t, err)
	assert.Equal(t, []envcopy.Source{{Src: "/parent/.env", Dst: "/wt/.env"}}, sources)
	assert.Empty(t, warnings)
}

func TestCandidatesAreEveryPathSourcesMayLookAt(t *testing.T) {
	files := []domain.EnvFile{{Target: ".env"}}

	example, err := envcopy.Candidates(envcopy.Request{Strategy: domain.EnvStrategyExample, Files: files, MainWorktreePath: "/main"})
	require.NoError(t, err)
	parent, err := envcopy.Candidates(envcopy.Request{Strategy: domain.EnvStrategyParent, Files: files, MainWorktreePath: "/main", ParentWorktreePath: "/parent"})
	require.NoError(t, err)

	assert.Equal(t, envcopy.TemplateCandidates("/main", files[0]), example)
	assert.Equal(t, []string{"/parent/.env", "/main/.env"}, parent)
}

func TestAnUnknownStrategyIsRefused(t *testing.T) {
	request := envcopy.Request{Strategy: "elsewhere", Files: []domain.EnvFile{{Target: ".env"}}}

	_, _, err := envcopy.Sources(envcopy.SourcesParams{Request: request})
	require.Error(t, err)
	_, err = envcopy.Candidates(request)
	require.Error(t, err)
}
