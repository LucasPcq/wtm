package runinit_test

import (
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/runinit"
	"github.com/LucasPcq/wtm/next/internal/ui/formtest"
	"github.com/stretchr/testify/require"
)

func TestAFirstRunInitWritesRunTomlAndRewritesCompose(t *testing.T) {
	w := newWorld(t)

	w.apply(t, runinit.Request{})

	formtest.Golden(t, "first.run.toml", w.runToml(t))
	formtest.Golden(t, "first.docker-compose.yml", readFile(t, filepath.Join(w.main, "docker-compose.yml")))
}

func TestTheProblemsToCheckOfAFirstRunInit(t *testing.T) {
	w := newWorld(t)

	questions, err := w.usecase.Questions(t.Context(), runinit.Request{})

	require.NoError(t, err)
	for _, p := range questions.Errors {
		t.Logf("%s %s %v", p.Path, p.Code, p.Params)
	}
	t.Logf("%+v", questions.Decision.Linkable)
}
