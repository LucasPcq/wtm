package output

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/styles"
)

// relTarget renders a worktree's target path relative to the repo (base_path +
// directory name) so the preview stays readable instead of showing absolute paths.
func relTarget(basePath, toPath string) string {
	return filepath.Join(basePath, filepath.Base(toPath))
}

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
		case domain.RelocateStatusBlockedDest:
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
				Danger(w, fmt.Sprintf("%s — target path already occupied: %s", step.Branch, step.ToPath))
			}
		})
	}
	if noops > 0 {
		section(func() {
			Message(w, styles.Muted.Render(fmt.Sprintf("%d worktree(s) already in place.", noops)))
		})
	}
	if adoptions > 0 {
		section(func() {
			Message(w, styles.Primary.Render(fmt.Sprintf("→ You'll choose a parent branch for %d worktree(s).", adoptions)))
		})
	}
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
	return fmt.Sprintf("%s %s %s%s", step.Branch, styles.Muted.Render("→"), relTarget(basePath, step.ToPath), suffix)
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
	var done, errored []domain.RelocateStepResult
	var skipped, blocked []string
	for _, step := range result.Steps {
		switch step.Status {
		case domain.RelocateStatusMoved, domain.RelocateStatusMovedAdopted, domain.RelocateStatusAdopted:
			done = append(done, step)
		case domain.RelocateStatusSkippedDirty, domain.RelocateStatusSkippedLocked:
			skipped = append(skipped, step.Branch)
		case domain.RelocateStatusBlockedDest:
			blocked = append(blocked, step.Branch)
		case domain.RelocateStatusError:
			errored = append(errored, step)
		}
	}

	hasIssue := len(blocked) > 0 || len(errored) > 0
	headline := Tally(
		TallyPart{Count: len(done), Label: domain.TallyApplied},
		TallyPart{Count: len(skipped), Label: domain.TallySkipped},
		TallyPart{Count: len(blocked) + len(errored), Label: domain.TallyBlocked},
	)
	if hasIssue {
		Warning(w, "Relocation finished with issues  "+styles.Muted.Render(headline))
	} else {
		Success(w, "Relocation complete  "+styles.Muted.Render(headline))
	}
	Blank(w)

	for _, step := range done {
		Success(w, resultDoneLine(result.BasePath, step))
	}
	if len(done) > 0 && (len(skipped) > 0 || len(blocked) > 0 || len(errored) > 0) {
		Blank(w)
	}

	if len(skipped) > 0 {
		Warning(w, fmt.Sprintf("Skipped: %s (re-run with --force)", strings.Join(skipped, ", ")))
	}
	if len(blocked) > 0 {
		Danger(w, fmt.Sprintf("Blocked: %s (target path occupied)", strings.Join(blocked, ", ")))
	}
	for _, step := range errored {
		Error(w, fmt.Sprintf("%s failed — %s", step.Branch, step.Detail))
	}

	if result.BasePathUpdated {
		Blank(w)
		Success(w, fmt.Sprintf("config base_path updated to %q", result.BasePath))
	}
}

func resultDoneLine(basePath string, step domain.RelocateStepResult) string {
	switch step.Status {
	case domain.RelocateStatusAdopted:
		return fmt.Sprintf("%s adopted in place (parent: %s)", step.Branch, step.Parent)
	case domain.RelocateStatusMovedAdopted:
		return fmt.Sprintf("%s %s %s (adopted, parent: %s)", step.Branch, styles.Muted.Render("→"), relTarget(basePath, step.ToPath), step.Parent)
	default:
		return fmt.Sprintf("%s %s %s", step.Branch, styles.Muted.Render("→"), relTarget(basePath, step.ToPath))
	}
}

// RelocateRecapParams holds inputs for SprintRelocateRecap.
type RelocateRecapParams struct {
	Plan    domain.RelocatePlan
	Parents map[string]string
	// PreviousBasePath, when non-empty and different from Plan.BasePath, prepends a
	// "base_path: <old> → <new>" header so the recap makes the reconfiguration explicit.
	PreviousBasePath string
}

// SprintRelocateRecap renders the interactive wizard's confirmation body: the
// resolved actions (with chosen adoption parents) grouped as To apply / Skipped /
// Blocked, optionally headed by the base_path change. It returns plain text (no outer
// padding); the wizard's recap frame owns the vertical spacing.
func SprintRelocateRecap(params RelocateRecapParams) string {
	var apply, skipped, blocked []string
	for _, step := range params.Plan.Steps {
		switch step.Status {
		case domain.RelocateStatusMove, domain.RelocateStatusAdopt:
			apply = append(apply, recapApplyLine(params.Plan.BasePath, step, params.Parents))
		case domain.RelocateStatusSkippedDirty:
			skipped = append(skipped, step.Branch+" — uncommitted changes")
		case domain.RelocateStatusSkippedLocked:
			skipped = append(skipped, step.Branch+" — locked")
		case domain.RelocateStatusBlockedDest:
			blocked = append(blocked, step.Branch+" — target path occupied")
		}
	}

	var b strings.Builder
	if params.PreviousBasePath != "" && params.PreviousBasePath != params.Plan.BasePath {
		b.WriteString(fmt.Sprintf("base_path: %s → %s\n\n", params.PreviousBasePath, params.Plan.BasePath))
	}
	writeRecapGroup(&b, "To apply:", apply)
	writeRecapGroup(&b, "Skipped:", skipped)
	writeRecapGroup(&b, "Blocked:", blocked)

	body := strings.TrimRight(b.String(), "\n")
	if len(apply)+len(skipped)+len(blocked) == 0 {
		if body != "" {
			// A base_path header is present: the config changes even with no worktree to move.
			body += "\n\nNo worktrees to move — base_path config will be updated."
		} else {
			body = "Nothing to relocate — everything is already in place."
		}
	}
	return body
}

func writeRecapGroup(b *strings.Builder, title string, lines []string) {
	if len(lines) == 0 {
		return
	}
	if b.Len() > 0 {
		b.WriteString("\n")
	}
	b.WriteString(title)
	b.WriteString("\n")
	for _, line := range lines {
		b.WriteString("  " + line + "\n")
	}
}

func recapApplyLine(basePath string, step domain.RelocateStep, parents map[string]string) string {
	if step.Status == domain.RelocateStatusAdopt {
		return fmt.Sprintf("%s  adopt in place (parent: %s)", step.Branch, recapResolveParent(step, parents))
	}
	target := relTarget(basePath, step.ToPath)
	if step.Adopt {
		return fmt.Sprintf("%s → %s (adopt, parent: %s)", step.Branch, target, recapResolveParent(step, parents))
	}
	return fmt.Sprintf("%s → %s", step.Branch, target)
}

func recapResolveParent(step domain.RelocateStep, parents map[string]string) string {
	if parent, ok := parents[step.Branch]; ok && parent != "" {
		return parent
	}
	return step.Parent
}

// WriteRelocateResultJSON writes the relocate result as pretty-printed JSON.
func WriteRelocateResultJSON(w io.Writer, result domain.RelocateResult) error {
	return encodeJSON(w, result)
}
