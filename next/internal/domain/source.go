package domain

import (
	"slices"

	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

// SourceOptions are the branches of repo a new branch can start from, those of except excepted: each local branch with where it stands against origin, the base branch marked, then each branch only origin has, offered as origin/<branch>.
func SourceOptions(repo worktree.Repository, except ...string) []Option {
	options := make([]Option, 0, len(repo.Branches.Local)+len(repo.Branches.Remote))
	for _, local := range repo.Branches.Local {
		if !slices.Contains(except, local.Name) {
			options = append(options, Option{Value: local.Name, Base: local.Name == repo.Config.Worktrees.BaseBranch, Origin: repo.OriginOf(local.Name)})
		}
	}

	for _, remote := range repo.Branches.Remote {
		if !repo.Local(remote.Name) && !slices.Contains(except, remote.Name) {
			options = append(options, Option{Value: worktree.OriginPrefix + remote.Name})
		}
	}

	return options
}
