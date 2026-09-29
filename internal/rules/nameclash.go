package rules

import "github.com/LucasPcq/wtm/internal/domain"

type WorktreeNameClashParams struct {
	Branch string
	// Live are the branches of the worktrees that exist, the main checkout's
	// included.
	Live []string
}

// WorktreeNameClash finds a live worktree whose derived names Branch would
// share: the slug names its compose project and its namespaces, the host label
// its proxy address, and each folds characters git keeps apart.
func WorktreeNameClash(params WorktreeNameClashParams) (domain.WorktreeNameClash, bool) {
	slug := WorktreeSlug(params.Branch)
	host := HostLabel(slug)
	for _, live := range params.Live {
		if live == "" || live == params.Branch {
			continue
		}
		other := WorktreeSlug(live)
		if other == slug {
			return domain.WorktreeNameClash{Branch: live, Name: slug}, true
		}
		if HostLabel(other) == host {
			return domain.WorktreeNameClash{Branch: live, Name: host}, true
		}
	}
	return domain.WorktreeNameClash{}, false
}
