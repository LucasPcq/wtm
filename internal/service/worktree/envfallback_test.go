package worktree

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra"
	"github.com/LucasPcq/wtm/internal/testutil/gittest"
)

func parentEnvConfig() domain.Config {
	cfg := domain.Config{}
	cfg.Project.Env.Strategy = domain.EnvStrategyParent
	cfg.Project.Env.Files = []domain.EnvFile{{Target: ".env"}}
	return cfg
}

func TestEnvParentFallsBackToMain(t *testing.T) {
	dir := gittest.InitRepo(t)
	mainBranch, err := infra.CurrentBranch(t.Context(), dir)
	if err != nil {
		t.Fatalf("current branch: %v", err)
	}

	// The main branch is checked out in the main worktree → no fallback.
	if EnvParentFallsBackToMain(t.Context(), EnvFallbackParams{ProjectDir: dir, Source: mainBranch, Config: parentEnvConfig()}) {
		t.Errorf("expected no fallback for a branch with a worktree (%s)", mainBranch)
	}

	// A branch with no worktree → fallback to main.
	if !EnvParentFallsBackToMain(t.Context(), EnvFallbackParams{ProjectDir: dir, Source: "ghost", Config: parentEnvConfig()}) {
		t.Error("expected fallback for a source branch without a worktree")
	}

	// Non-parent strategy never falls back.
	cfg := parentEnvConfig()
	cfg.Project.Env.Strategy = domain.EnvStrategyMain
	if EnvParentFallsBackToMain(t.Context(), EnvFallbackParams{ProjectDir: dir, Source: "ghost", Config: cfg}) {
		t.Error("main strategy must not be treated as a parent fallback")
	}
}
