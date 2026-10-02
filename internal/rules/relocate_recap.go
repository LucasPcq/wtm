package rules

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
)

// RelocateTargetLabel renders a worktree's target relative to the repo
// (base_path + directory name), which reads where an absolute path does not.
func RelocateTargetLabel(basePath, toPath string) string {
	return filepath.Join(basePath, filepath.Base(toPath))
}

func RelocateBlockedLine(step domain.RelocateStep) string {
	switch step.Status {
	case domain.RelocateStatusBlockedJobs:
		return fmt.Sprintf(domain.RelocateBlockedJobsFmt, step.Branch, step.Branch)
	case domain.RelocateStatusBlockedName:
		return step.Detail
	default:
		return fmt.Sprintf("%s — target path already occupied: %s", step.Branch, step.ToPath)
	}
}

type RelocateRecapParams struct {
	Plan    domain.RelocatePlan
	Parents map[string]string
	// PreviousBasePath, when non-empty and different from Plan.BasePath, prepends a
	// "base_path: <old> → <new>" header so the recap makes the reconfiguration explicit.
	PreviousBasePath string
}

// RelocateRecap renders the wizard's confirmation body: the
// resolved actions (with chosen adoption parents) grouped as To apply / Skipped /
// Blocked, optionally headed by the base_path change. It returns plain text (no outer
// padding); the wizard's recap frame owns the vertical spacing.
func RelocateRecap(params RelocateRecapParams) string {
	var apply, skipped, blocked []string
	for _, step := range params.Plan.Steps {
		switch step.Status {
		case domain.RelocateStatusMove, domain.RelocateStatusAdopt:
			apply = append(apply, relocateRecapApplyLine(params.Plan.BasePath, step, params.Parents))
		case domain.RelocateStatusSkippedDirty:
			skipped = append(skipped, step.Branch+" — uncommitted changes")
		case domain.RelocateStatusSkippedLocked:
			skipped = append(skipped, step.Branch+" — locked")
		case domain.RelocateStatusBlockedDest:
			blocked = append(blocked, step.Branch+" — target path occupied")
		case domain.RelocateStatusBlockedJobs, domain.RelocateStatusBlockedName:
			blocked = append(blocked, RelocateBlockedLine(step))
		}
	}

	var b strings.Builder
	if params.PreviousBasePath != "" && params.PreviousBasePath != params.Plan.BasePath {
		b.WriteString(fmt.Sprintf("base_path: %s → %s\n", params.PreviousBasePath, params.Plan.BasePath))
	}
	writeRelocateRecapGroup(&b, "To apply:", apply)
	writeRelocateRecapGroup(&b, "Skipped:", skipped)
	writeRelocateRecapGroup(&b, "Blocked:", blocked)

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

func writeRelocateRecapGroup(b *strings.Builder, title string, lines []string) {
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

func relocateRecapApplyLine(basePath string, step domain.RelocateStep, parents map[string]string) string {
	if step.Status == domain.RelocateStatusAdopt {
		return fmt.Sprintf("%s  adopt in place (parent: %s)", step.Branch, relocateRecapParent(step, parents))
	}
	target := RelocateTargetLabel(basePath, step.ToPath)
	if step.Adopt {
		return fmt.Sprintf("%s → %s (adopt, parent: %s)", step.Branch, target, relocateRecapParent(step, parents))
	}
	return fmt.Sprintf("%s → %s", step.Branch, target)
}

func relocateRecapParent(step domain.RelocateStep, parents map[string]string) string {
	if parent, ok := parents[step.Branch]; ok && parent != "" {
		return parent
	}
	return step.Parent
}
