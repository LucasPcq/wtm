package envcopy

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/wtm/project"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSourcesNameWhatTheyCouldNotFind(t *testing.T) {
	present := map[string]bool{"/main/.env": true}

	example, exampleWarnings := sources(t, SourcesParams{Request: Request{Strategy: project.EnvExample, Files: []project.EnvFile{{Target: ".env", Template: ".env.example"}}, TargetDir: "/wt", MainPath: "/main"}, Present: present})
	missing, missingWarnings := sources(t, SourcesParams{Request: Request{Strategy: project.EnvMain, Files: []project.EnvFile{{Target: ".env.local"}}, TargetDir: "/wt", MainPath: "/main"}, Present: present})
	fallback, fallbackWarnings := sources(t, SourcesParams{Request: Request{Strategy: project.EnvParent, Files: []project.EnvFile{{Target: ".env"}, {Target: ".env.local"}}, TargetDir: "/wt", MainPath: "/main", ParentPath: "/parent"}, Present: present})

	assert.Empty(t, example)
	assert.Equal(t, []Warning{NoTemplate{Target: ".env"}}, exampleWarnings)
	assert.Empty(t, missing)
	assert.Equal(t, []Warning{SourceMissing{Target: ".env.local", Dir: "/main"}}, missingWarnings)
	assert.Equal(t, []Source{{From: "/main/.env", To: "/wt/.env"}}, fallback)
	assert.Equal(t, []Warning{
		ParentFallback{Target: ".env"},
		ParentFallback{Target: ".env.local"},
		SourceMissing{Target: ".env.local", Dir: "/main"},
	}, fallbackWarnings)
}

func TestTheMainStrategyCopiesFromTheMainCheckout(t *testing.T) {
	request := Request{Strategy: project.EnvMain, Files: []project.EnvFile{{Target: "apps/api/.env"}}, TargetDir: "/wt", MainPath: "/main"}

	found, warnings := sources(t, SourcesParams{Request: request, Present: map[string]bool{"/main/apps/api/.env": true}})

	assert.Equal(t, []Source{{From: "/main/apps/api/.env", To: "/wt/apps/api/.env"}}, found)
	assert.Empty(t, warnings)
}

func TestTheParentStrategyCopiesFromTheParentWhenItHoldsTheFile(t *testing.T) {
	request := Request{Strategy: project.EnvParent, Files: []project.EnvFile{{Target: ".env"}}, TargetDir: "/wt", MainPath: "/main", ParentPath: "/parent"}

	found, warnings := sources(t, SourcesParams{Request: request, Present: map[string]bool{"/parent/.env": true, "/main/.env": true}})

	assert.Equal(t, []Source{{From: "/parent/.env", To: "/wt/.env"}}, found)
	assert.Empty(t, warnings)
}

func TestTheParentStrategyWithoutAParentWorktreeCopiesFromMainSilently(t *testing.T) {
	request := Request{Strategy: project.EnvParent, Files: []project.EnvFile{{Target: ".env"}}, TargetDir: "/wt", MainPath: "/main"}

	found, warnings := sources(t, SourcesParams{Request: request, Present: map[string]bool{"/main/.env": true}})

	assert.Equal(t, []Source{{From: "/main/.env", To: "/wt/.env"}}, found)
	assert.Empty(t, warnings)
}

func TestCandidatesAreEveryPathSourcesMayLookAt(t *testing.T) {
	files := []project.EnvFile{{Target: ".env"}}

	example, err := Candidates(Request{Strategy: project.EnvExample, Files: files, MainPath: "/main"})
	require.NoError(t, err)
	parent, err := Candidates(Request{Strategy: project.EnvParent, Files: files, MainPath: "/main", ParentPath: "/parent"})
	require.NoError(t, err)
	main, err := Candidates(Request{Strategy: project.EnvMain, Files: files, MainPath: "/main"})
	require.NoError(t, err)

	assert.Equal(t, []string{"/main/.env.example", "/main/.env.dist", "/main/.env.sample", "/main/.env.template", "/main/.env.tmpl"}, example)
	assert.Equal(t, []string{"/parent/.env", "/main/.env"}, parent)
	assert.Equal(t, []string{"/main/.env"}, main)
}

func TestAnUnknownStrategyIsRefused(t *testing.T) {
	request := Request{Strategy: "elsewhere", Files: []project.EnvFile{{Target: ".env"}}}

	_, _, err := Sources(SourcesParams{Request: request})
	assert.EqualError(t, err, "unknown env strategy: elsewhere")
	_, err = Candidates(request)
	assert.EqualError(t, err, "unknown env strategy: elsewhere")
}
