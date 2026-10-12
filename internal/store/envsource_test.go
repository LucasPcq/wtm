package store_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/store"
)

func existing(paths ...string) func(string) bool {
	return func(path string) bool {
		for _, p := range paths {
			if p == path {
				return true
			}
		}
		return false
	}
}

func TestCopySourcesNameWhatTheyCouldNotFind(t *testing.T) {
	exists := existing("/main/.env")
	sources := func(request store.EnvCopy) ([]store.EnvSource, []store.EnvWarning) {
		t.Helper()
		found, warnings, err := store.CopySources(store.CopySourcesParams{Request: request, Exists: exists})
		require.NoError(t, err)
		return found, warnings
	}

	exampleSources, example := sources(store.EnvCopy{Strategy: domain.EnvStrategyExample, Files: []domain.EnvFile{{Target: ".env", Template: ".env.example"}}, TargetDir: "/wt", MainWorktreePath: "/main"})
	missingSources, missing := sources(store.EnvCopy{Strategy: domain.EnvStrategyMain, Files: []domain.EnvFile{{Target: ".env.local"}}, TargetDir: "/wt", MainWorktreePath: "/main"})
	fallbackSources, fallback := sources(store.EnvCopy{Strategy: domain.EnvStrategyParent, Files: []domain.EnvFile{{Target: ".env"}, {Target: ".env.local"}}, TargetDir: "/wt", MainWorktreePath: "/main", ParentWorktreePath: "/parent"})

	assert.Empty(t, exampleSources)
	assert.Equal(t, []store.EnvWarning{store.NoTemplate{Target: ".env"}}, example)
	assert.Empty(t, missingSources)
	assert.Equal(t, []store.EnvWarning{store.SourceMissing{Target: ".env.local", Dir: "/main"}}, missing)
	assert.Equal(t, []store.EnvSource{{Src: "/main/.env", Dst: "/wt/.env"}}, fallbackSources)
	assert.Equal(t, []store.EnvWarning{
		store.ParentFallback{Target: ".env"},
		store.ParentFallback{Target: ".env.local"},
		store.SourceMissing{Target: ".env.local", Dir: "/main"},
	}, fallback)
}

func TestCopySourcesRefuseAnUnknownStrategy(t *testing.T) {
	_, _, err := store.CopySources(store.CopySourcesParams{Request: store.EnvCopy{Strategy: "elsewhere", Files: []domain.EnvFile{{Target: ".env"}}}, Exists: existing()})

	require.Error(t, err)
}

func TestTemplateSourceIsThePinnedTemplateElseTheFirstCandidate(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".env.sample"), "")

	assert.Equal(t, filepath.Join(dir, ".env.sample"), store.TemplateSource(dir, domain.EnvFile{Target: ".env"}))
	assert.Empty(t, store.TemplateSource(dir, domain.EnvFile{Target: ".env", Template: ".env.example"}), "a pinned template that is gone is not replaced by a guess")
}
