package cli

import (
	"strconv"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/command/prune"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
	"github.com/LucasPcq/wtm/next/internal/ui/output"
)

// words are how the command line words each field of prune's form; the worktrees' options and the switches name what they decide on the questions observed: why each worktree is done with, the children that would move, the worktrees picked that are unsafe.
func words(q prune.Questions, field domain.Field) form.Words {
	switch field.Spec().Path {
	case domain.PathWorktree:
		return form.Words{Flag: "<worktree>", Label: "Worktrees", Help: "The worktrees to prune, among those done with; none for every one safe to remove", Empty: "Select at least one worktree", Options: candidateWords(q)}
	case prune.PathReparentChildren:
		return form.Words{Flag: "--reparent-children", Label: "Reparent children", Help: reparentHelp(q), Options: map[string]form.OptionWords{
			"true":  {Label: "Reparent them", Short: "reparent"},
			"false": {Label: "Leave them orphaned", Short: "leave orphaned"},
		}}
	case prune.PathForce:
		return form.Words{Flag: "--force", Label: "Force", Help: forceHelp(q), Options: map[string]form.OptionWords{
			"true":  {Label: "Yes, prune them anyway", Short: "forced"},
			"false": {Label: "No", Short: "not forced"},
		}}
	case prune.PathDropData:
		return form.Words{Flag: "--drop-data", Label: "Data", Help: dropDataHelp(q), Options: map[string]form.OptionWords{
			"true":  {Label: "Start " + services(q) + " and drop it now", Short: "drop now"},
			"false": {Label: "Keep it until " + services(q) + " next starts", Short: "drop later"},
		}}
	case prune.PathKeepData:
		return form.Words{Flag: "--keep-data", Label: "Keep data", Help: "Keep the namespaces the pruned worktrees made in shared services (databases, buckets): nothing is dropped"}
	case prune.PathMerged:
		return form.Words{Flag: "--merged", Label: "Merged", Help: "Prune the worktrees whose pull request merged (none of --merged, --closed, --gone asks for all three)"}
	case prune.PathClosed:
		return form.Words{Flag: "--closed", Label: "Closed", Help: "Prune the worktrees whose pull request was closed unmerged"}
	case prune.PathGone:
		return form.Words{Flag: "--gone", Label: "Gone", Help: "Prune the worktrees whose branch origin deleted"}
	case prune.PathNoFetch:
		return form.Words{Flag: "--no-fetch", Label: "No fetch", Help: "Read origin's branches as last fetched, for --gone"}
	}

	return form.Words{}
}

// candidateWords label each worktree done with by why it is, and what makes it unsafe to remove.
func candidateWords(q prune.Questions) map[string]form.OptionWords {
	said := map[string]form.OptionWords{}
	for _, c := range q.Candidates {
		label := c.Worktree + " — " + reasonWords(c)
		if len(c.Blockers) > 0 {
			label += " · " + output.Blockers(c.Blockers)
		}
		said[c.Worktree] = form.OptionWords{Label: label}
	}

	return said
}

func reasonWords(c prune.Candidate) string {
	switch {
	case c.Reason == prune.ReasonGone:
		return "deleted on origin"
	case c.PR == nil:
		return string(c.Reason)
	}

	return string(c.Reason) + " #" + strconv.Itoa(c.PR.Number)
}

func reparentHelp(q prune.Questions) string {
	if len(q.Orphans) == 0 {
		return "Move the children a pruned worktree leaves behind onto its nearest surviving ancestor, else the base branch"
	}

	lines := []string{"These children would be left orphaned:"}
	for _, move := range q.Orphans {
		lines = append(lines, "  "+move.Branch+" would rebase onto "+move.To+" instead of "+move.From)
	}
	return strings.Join(lines, "\n")
}

func dropDataHelp(q prune.Questions) string {
	if len(q.Down) == 0 {
		return "Start a shared service that is down to drop the namespaces the pruned worktrees made in it now, rather than once it next starts"
	}
	return "The pruned worktrees made data in " + services(q) + ", which is down: start it to drop that data now, or keep it until it next starts (then run up or prune drops it)"
}

// services names the shared services down that the data waits on, "the service" when none is known.
func services(q prune.Questions) string {
	if len(q.Down) == 0 {
		return "the service"
	}
	return strings.Join(q.Down, ", ")
}

func forceHelp(q prune.Questions) string {
	if len(q.Unsafe) == 0 {
		return "Prune a worktree that is locked, has uncommitted changes, unpushed commits or an open pull request, or whose jobs would not stop"
	}

	lines := []string{"These worktrees are unsafe to remove:"}
	for _, c := range q.Unsafe {
		lines = append(lines, "  "+c.Worktree+" — "+output.Blockers(c.Blockers))
	}
	return strings.Join(lines, "\n")
}
