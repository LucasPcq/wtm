package envcopy

import "github.com/LucasPcq/wtm/next/internal/wtm/project"

type ParentFallbackParams struct {
	Strategy          project.EnvStrategy
	HasFiles          bool
	SourceHasWorktree bool
}

func ParentFallsBackToMain(params ParentFallbackParams) bool {
	return params.HasFiles && params.Strategy == project.EnvParent && !params.SourceHasWorktree
}
