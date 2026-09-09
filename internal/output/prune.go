package output

import (
	"fmt"
	"io"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/styles"
)

// FormatPrunePlan renders the dry-run / preview of a prune: the worktrees that
// would be removed (with their reason), the reparenting of surviving children,
// and anything skipped. Raw body — the command's frame owns the outer padding.
func FormatPrunePlan(w io.Writer, plan domain.PrunePlan) {
	if len(plan.Selected) == 0 {
		Message(w, "Nothing to prune.")
	} else {
		items := make([]AnnounceItem, 0, len(plan.Selected))
		for _, c := range plan.Selected {
			items = append(items, AnnounceItem{Label: c.Branch, Value: rules.PruneReasonLabel(c.Reason)})
		}
		Announce(w, fmt.Sprintf("Would prune %d worktree(s):", len(plan.Selected)), items)
	}

	if len(plan.Reparents) > 0 {
		Blank(w)
		items := make([]AnnounceItem, 0, len(plan.Reparents))
		for _, r := range plan.Reparents {
			items = append(items, AnnounceItem{Label: r.Branch, Value: fmt.Sprintf("%s → %s", r.OldParent, r.NewParent)})
		}
		Announce(w, "Reparent orphaned children onto their grandparent:", items)
	}

	if len(plan.Skipped) > 0 {
		Blank(w)
		items := make([]AnnounceItem, 0, len(plan.Skipped))
		for _, s := range plan.Skipped {
			items = append(items, AnnounceItem{Label: s.Branch, Value: rules.PruneReasonLabel(s.Reason)})
		}
		Announce(w, "Skipped:", items)
	}
}

// FormatPruneResult renders the outcome of an executed prune: what was removed,
// counted and named on one line, then a line for each thing the reader still has
// to deal with. A prune names what it destroyed — knowing what is gone is
// actionable — but the picker and the recap have already shown that list twice,
// so it does not get a line each. Raw body — the command's frame owns the padding.
func FormatPruneResult(w io.Writer, result domain.PruneResult) {
	if len(result.Pruned) == 0 {
		Unchanged(w, domain.PruneNothingToPrune)
	}
	if len(result.Pruned) > 0 {
		Success(w, Tally(
			TallyPart{Count: len(result.Pruned), Label: domain.TallyPruned},
			TallyPart{Count: len(result.Reparented), Label: domain.TallyReparented},
			TallyPart{Count: len(result.Skipped), Label: domain.TallySkipped},
		))
		Message(w, styles.Muted.Render(strings.Join(rules.PrunedBranches(result), ", ")))
	}
	// Which parent a child was moved onto is not accounting: its next `wtm sync`
	// rebases onto that branch.
	if len(result.Reparented) > 0 {
		Message(w, styles.Muted.Render(strings.Join(rules.ReparentedPairs(result.Reparented), ", ")))
	}
	for _, o := range result.Orphaned {
		Warning(w, fmt.Sprintf("%s still points at the removed parent %s — reparent it with `wtm reparent`", o.Branch, o.OldParent))
	}
	for _, s := range result.Skipped {
		Warning(w, fmt.Sprintf(domain.PruneSkippedFmt, s.Branch, rules.PruneReasonLabel(s.Reason)))
	}
}

// WritePruneResultJSON writes the prune outcome as JSON, ensuring the slices
// marshal as [] rather than null.
func WritePruneResultJSON(w io.Writer, result domain.PruneResult) error {
	if result.Pruned == nil {
		result.Pruned = []domain.PruneCandidate{}
	}
	if result.Reparented == nil {
		result.Reparented = []domain.ReparentResult{}
	}
	if result.Orphaned == nil {
		result.Orphaned = []domain.ReparentResult{}
	}
	if result.Skipped == nil {
		result.Skipped = []domain.PruneSkip{}
	}
	return encodeJSON(w, result)
}
