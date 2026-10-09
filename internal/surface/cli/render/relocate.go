package render

import (
	"fmt"
	"io"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/styles"
)

// FormatRelocatePlan prints the planned actions grouped by category (to apply,
// skipped, blocked) with blank lines between groups, then a note when the user
// will be asked to choose adoption parents. It emits a raw body with no leading
// blank line; the caller's frame owns the outer vertical padding.
func FormatRelocatePlan(w io.Writer, plan domain.RelocatePlan) {
	var apply, skipped, blocked []domain.RelocateStep
	adoptions, noops := 0, 0
	for _, step := range plan.Steps {
		switch step.Status {
		case domain.RelocateStatusMove, domain.RelocateStatusAdopt:
			apply = append(apply, step)
			if step.Adopt {
				adoptions++
			}
		case domain.RelocateStatusSkippedDirty, domain.RelocateStatusSkippedLocked:
			skipped = append(skipped, step)
		case domain.RelocateStatusBlockedDest, domain.RelocateStatusBlockedJobs, domain.RelocateStatusBlockedName:
			blocked = append(blocked, step)
		case domain.RelocateStatusNoop:
			noops++
		}
	}

	// Sections are separated by a blank line but none trails the last one, so the
	// caller controls the spacing to whatever follows (the wizard, or "Dry run").
	section := newSectionWriter(w)

	if len(apply) > 0 {
		section(func() {
			SectionTitle(w, fmt.Sprintf("To apply (%d)", len(apply)))
			for _, step := range apply {
				Message(w, styles.Muted.Render("• ")+planApplyLine(plan.BasePath, step))
			}
		})
	}
	if len(skipped) > 0 {
		section(func() {
			SectionTitle(w, fmt.Sprintf("Skipped (%d)", len(skipped)))
			for _, step := range skipped {
				Warning(w, planSkipLine(step))
			}
		})
	}
	if len(blocked) > 0 {
		section(func() {
			SectionTitle(w, fmt.Sprintf("Blocked (%d)", len(blocked)))
			for _, step := range blocked {
				Error(w, rules.RelocateBlockedLine(step))
			}
		})
	}
	if noops > 0 {
		section(func() {
			Unchanged(w, fmt.Sprintf("%d worktree(s) already in place.", noops))
		})
	}
	if adoptions > 0 {
		section(func() {
			Message(w, styles.Primary.Render(fmt.Sprintf(domain.RelocateAdoptionsNoteFmt, adoptions, plan.BaseBranch)))
		})
	}
}

type RelocatePreviewParams struct {
	Plan         domain.RelocatePlan
	FromBasePath string
}

// FormatRelocatePreview is a dry run's body: the plan, or the base_path rewrite
// alone when no worktree has to move.
func FormatRelocatePreview(w io.Writer, params RelocatePreviewParams) {
	if !rules.PlanHasWork(params.Plan) {
		Message(w, fmt.Sprintf(domain.RelocateBasePathOnlyFmt, params.FromBasePath, params.Plan.BasePath))
		return
	}
	if params.FromBasePath != params.Plan.BasePath {
		Message(w, fmt.Sprintf(domain.RelocateBasePathChangeFmt, params.FromBasePath, params.Plan.BasePath))
		Blank(w)
	}
	FormatRelocatePlan(w, params.Plan)
}

// newSectionWriter returns a function that renders a section, inserting a blank
// line before every section except the first. No blank trails the last section.
func newSectionWriter(w io.Writer) func(render func()) {
	first := true
	return func(render func()) {
		if !first {
			Blank(w)
		}
		first = false
		render()
	}
}

func planApplyLine(basePath string, step domain.RelocateStep) string {
	if step.Status == domain.RelocateStatusAdopt {
		return fmt.Sprintf("%s %s", step.Branch, styles.Muted.Render("(adopt in place)"))
	}
	suffix := ""
	if step.Adopt {
		suffix = styles.Muted.Render(" (+ adopt)")
	}
	return fmt.Sprintf("%s %s %s%s", step.Branch, styles.Muted.Render(domain.MoveArrowGlyph), rules.RelocateTargetLabel(basePath, step.ToPath), suffix)
}

func planSkipLine(step domain.RelocateStep) string {
	if step.Status == domain.RelocateStatusSkippedLocked {
		return fmt.Sprintf("%s — worktree is locked (use --force)", step.Branch)
	}
	return fmt.Sprintf("%s — uncommitted changes (use --force)", step.Branch)
}

// FormatRelocateResult prints the outcome as a clear conclusion: a headline
// (success or "with issues") with a one-line tally, the detail of what was
// actually applied (with parents), then condensed skipped/blocked lines. It is
// deliberately distinct from FormatRelocatePlan so the end state doesn't read
// like a repeat of the opening preview. It emits a raw body with no trailing
// blank line; the caller's frame owns the outer vertical padding.
func FormatRelocateResult(w io.Writer, result domain.RelocateResult) {
	var done, errored, refused []domain.RelocateStepResult
	var skipped, blocked []string
	for _, step := range result.Steps {
		switch step.Status {
		case domain.RelocateStatusMoved, domain.RelocateStatusMovedAdopted, domain.RelocateStatusAdopted:
			done = append(done, step)
		case domain.RelocateStatusSkippedDirty, domain.RelocateStatusSkippedLocked:
			skipped = append(skipped, step.Branch)
		case domain.RelocateStatusBlockedDest:
			blocked = append(blocked, step.Branch)
		case domain.RelocateStatusBlockedJobs, domain.RelocateStatusBlockedName:
			refused = append(refused, step)
		case domain.RelocateStatusError:
			errored = append(errored, step)
		}
	}

	hasIssue := len(blocked) > 0 || len(errored) > 0 || len(refused) > 0
	if len(done)+len(skipped) == 0 && !hasIssue {
		if result.BasePathUpdated {
			Success(w, fmt.Sprintf("config base_path updated to %q", result.BasePath))
		}
		return
	}
	headline := rules.Tally(
		domain.TallyPart{Count: len(done), Label: domain.TallyApplied},
		domain.TallyPart{Count: len(skipped), Label: domain.TallySkipped},
		domain.TallyPart{Count: len(blocked) + len(errored) + len(refused), Label: domain.TallyBlocked},
	)
	switch {
	case hasIssue:
		Warning(w, "Relocation finished with issues  "+headline)
	case len(done) == 0:
		// Only skips: nothing moved, and each one waits on --force.
		Warning(w, domain.RelocateNothingAppliedPrefix+headline)
	default:
		Success(w, "Relocation complete  "+headline)
	}
	Blank(w)

	for _, step := range done {
		Success(w, resultDoneLine(result.BasePath, step))
	}
	if len(done) > 0 && (hasIssue || len(skipped) > 0) {
		Blank(w)
	}

	if len(skipped) > 0 {
		Warning(w, fmt.Sprintf("Skipped: %s (re-run with --force)", strings.Join(skipped, ", ")))
	}
	if len(blocked) > 0 {
		// Blocked is not skipped: --force does not lift it, so the move failed
		// rather than being held back.
		Error(w, fmt.Sprintf("Blocked: %s (target path occupied)", strings.Join(blocked, ", ")))
	}
	for _, step := range refused {
		Error(w, resultBlockedLine(step))
	}
	for _, step := range errored {
		Error(w, fmt.Sprintf("%s failed — %s", step.Branch, step.Detail))
	}

	if result.BasePathUpdated {
		Blank(w)
		Success(w, fmt.Sprintf("config base_path updated to %q", result.BasePath))
	}
}

func resultBlockedLine(step domain.RelocateStepResult) string {
	if step.Status == domain.RelocateStatusBlockedName {
		return step.Detail
	}
	return fmt.Sprintf(domain.RelocateBlockedJobsFmt, step.Branch, step.Branch)
}

func resultDoneLine(basePath string, step domain.RelocateStepResult) string {
	switch step.Status {
	case domain.RelocateStatusAdopted:
		return fmt.Sprintf("%s adopted in place (parent: %s)", step.Branch, step.Parent)
	case domain.RelocateStatusMovedAdopted:
		return fmt.Sprintf("%s %s %s (adopted, parent: %s)", step.Branch, styles.Muted.Render(domain.MoveArrowGlyph), rules.RelocateTargetLabel(basePath, step.ToPath), step.Parent)
	default:
		return fmt.Sprintf("%s %s %s", step.Branch, styles.Muted.Render(domain.MoveArrowGlyph), rules.RelocateTargetLabel(basePath, step.ToPath))
	}
}

// WriteRelocateResultJSON writes the relocate result as pretty-printed JSON.
func WriteRelocateResultJSON(w io.Writer, result domain.RelocateResult) error {
	return encodeJSON(w, result)
}
