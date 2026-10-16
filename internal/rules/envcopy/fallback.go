package envcopy

import "github.com/LucasPcq/wtm/internal/domain"

type ParentFallbackParams struct {
	Strategy          domain.EnvStrategy
	HasCopyFiles      bool
	SourceHasWorktree bool
}

func ParentFallsBackToMain(params ParentFallbackParams) bool {
	return params.HasCopyFiles && params.Strategy == domain.EnvStrategyParent && !params.SourceHasWorktree
}
