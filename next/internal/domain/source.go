package domain

import (
	"slices"

	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

// SourceOptions are the branches of snapshot a new branch can start from, those of except excepted: each local branch with its badge, then each branch only origin has, offered as origin/<branch>.
func SourceOptions(snapshot worktree.Snapshot, except ...string) []Option {
	options := make([]Option, 0, len(snapshot.Branches.Local)+len(snapshot.Branches.Remote))
	for _, local := range snapshot.Branches.Local {
		if !slices.Contains(except, local.Name) {
			options = append(options, Option{Value: local.Name, Badge: snapshot.Badge(local.Name)})
		}
	}

	for _, remote := range snapshot.Branches.Remote {
		if !snapshot.Local(remote.Name) && !slices.Contains(except, remote.Name) {
			options = append(options, Option{Value: worktree.OriginPrefix + remote.Name})
		}
	}

	return options
}
