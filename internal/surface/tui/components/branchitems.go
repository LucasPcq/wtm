package components

import (
	"fmt"

	"github.com/LucasPcq/wtm/internal/domain"
)

// BranchItemsParams holds inputs for BranchItems.
type BranchItemsParams struct {
	// Candidates are the branches to offer, already ordered (locals then remotes).
	Candidates []domain.BranchCandidate
	// Pinned is the value placed first and labelled with PinnedSuffix (e.g. the
	// default or base branch). Empty pins nothing. If it is not among Candidates
	// it is still added first, as a local item, so a detected default always shows.
	Pinned string
	// PinnedSuffix is appended to the pinned label (e.g. " (default)", " (base)").
	PinnedSuffix string
	// Exclude is a value omitted entirely (e.g. the worktree being reparented).
	Exclude string
	// ExcludeSet omits several values at once (e.g. every worktree selected for a
	// batch reparent, none of which may be its own parent). Empty omits nothing.
	ExcludeSet []string
}

// excludes reports whether a candidate name is filtered out by Exclude or ExcludeSet.
func (p BranchItemsParams) excludes(name string) bool {
	if name == p.Exclude {
		return true
	}
	for _, e := range p.ExcludeSet {
		if name == e {
			return true
		}
	}
	return false
}

// BranchItems builds picker rows from branch candidates with a consistent layout:
// the pinned branch first, then the remaining locals, then a separator followed by
// the remote-only branches (each tagged with a "remote" badge). Sharing this across
// the create, checkout, reparent and relocate pickers keeps remote-branch display
// identical everywhere.
func BranchItems(params BranchItemsParams) []SelectItem {
	items := make([]SelectItem, 0, len(params.Candidates)+2)

	pinnedFound := false
	for _, c := range params.Candidates {
		if c.Name == params.Pinned {
			pinnedFound = true
			items = append(items, branchItem(branchItemParams{candidate: c, suffix: params.PinnedSuffix}))
			break
		}
	}
	if params.Pinned != "" && !pinnedFound {
		items = append(items, branchItem(branchItemParams{candidate: domain.BranchCandidate{Name: params.Pinned}, suffix: params.PinnedSuffix}))
	}

	for _, c := range params.Candidates {
		if c.IsRemote || c.Name == params.Pinned || params.excludes(c.Name) {
			continue
		}
		items = append(items, branchItem(branchItemParams{candidate: c}))
	}

	hasRemote := false
	for _, c := range params.Candidates {
		if c.IsRemote && c.Name != params.Pinned && !params.excludes(c.Name) {
			hasRemote = true
			break
		}
	}
	if !hasRemote {
		return items
	}

	items = append(items, SelectItem{Separator: true})
	for _, c := range params.Candidates {
		if !c.IsRemote || c.Name == params.Pinned || params.excludes(c.Name) {
			continue
		}
		items = append(items, branchItem(branchItemParams{candidate: c}))
	}
	return items
}

type branchItemParams struct {
	candidate domain.BranchCandidate
	suffix    string
}

func branchItem(params branchItemParams) SelectItem {
	c := params.candidate
	item := SelectItem{Label: c.Name + params.suffix, Value: c.Name}
	if c.IsRemote {
		item.Badges = append(item.Badges, Badge{Text: domain.BadgeTextRemote, Variant: BadgeNeutral})
	}
	if badge, ok := DivergenceBadge(DivergenceBadgeParams{State: c.State, Ahead: c.Ahead, Behind: c.Behind}); ok {
		item.Badges = append(item.Badges, badge)
	}
	return item
}

// DivergenceBadgeParams holds inputs for DivergenceBadge.
type DivergenceBadgeParams struct {
	State  domain.DivergenceState
	Ahead  int
	Behind int
	// Label, when non-empty, is prefixed to the badge text to name the referential
	// the counts are measured against (e.g. "origin ↑2 ↓5"). Empty renders the bare
	// glyphs, as branch pickers do (a single, implicit origin referential).
	Label string
}

// DivergenceBadge renders an ahead/behind state as a compact tag (e.g. "↓5",
// "↑2 ↓5", or "origin ↑2 ↓5" when Label is set). Up-to-date and unknown (no origin
// counterpart) show none.
func DivergenceBadge(params DivergenceBadgeParams) (Badge, bool) {
	prefix := ""
	if params.Label != "" {
		prefix = params.Label + " "
	}
	switch params.State {
	case domain.DivergenceBehind:
		return Badge{Text: fmt.Sprintf("%s%s%d", prefix, domain.BadgeGlyphBehind, params.Behind), Variant: BadgeWarning}, true
	case domain.DivergenceAhead:
		return Badge{Text: fmt.Sprintf("%s%s%d", prefix, domain.BadgeGlyphAhead, params.Ahead), Variant: BadgeNeutral}, true
	case domain.DivergenceDiverged:
		return Badge{Text: fmt.Sprintf("%s%s%d %s%d", prefix, domain.BadgeGlyphAhead, params.Ahead, domain.BadgeGlyphBehind, params.Behind), Variant: BadgeDanger}, true
	default:
		return Badge{}, false
	}
}
