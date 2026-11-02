package cli

import (
	"strings"

	"github.com/LucasPcq/wtm/next/internal/command/clean"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
	"github.com/LucasPcq/wtm/next/internal/ui/output"
)

// words are how the command line words each field of clean's form; the switches name what they decide on the questions observed: the children that would move, the worktrees that are unsafe, the services down that their data waits on.
func words(q clean.Questions, field domain.Field) form.Words {
	switch field.Spec().Path {
	case domain.PathWorktree:
		return form.Words{Flag: "<worktree>", Label: "Worktrees", Help: "The worktrees to remove, by branch, or by path on no branch", Empty: "Select at least one worktree"}
	case clean.PathReparentChildren:
		return form.Words{Flag: "--reparent-children", Label: "Reparent children", Help: reparentHelp(q), Options: map[string]form.OptionWords{
			"true":  {Label: "Reparent them", Short: "reparent"},
			"false": {Label: "Leave them orphaned", Short: "leave orphaned"},
		}}
	case clean.PathForce:
		return form.Words{Flag: "--force", Label: "Force", Help: forceHelp(q), Options: map[string]form.OptionWords{
			"true":  {Label: "Yes, remove them anyway", Short: "forced"},
			"false": {Label: "No", Short: "not forced"},
		}}
	case clean.PathDropData:
		return form.Words{Flag: "--drop-data", Label: "Data", Help: dropDataHelp(q), Options: map[string]form.OptionWords{
			"true":  {Label: "Start " + services(q) + " and drop it now", Short: "drop now"},
			"false": {Label: "Keep it until " + services(q) + " next starts", Short: "drop later"},
		}}
	case clean.PathKeepData:
		return form.Words{Flag: "--keep-data", Label: "Keep data", Help: "Keep the namespaces the removed worktrees made in shared services (databases, buckets): nothing is dropped"}
	}

	return form.Words{}
}

func reparentHelp(q clean.Questions) string {
	if len(q.Orphans) == 0 {
		return "Move the children a removed worktree leaves behind onto its nearest surviving ancestor, else the base branch"
	}

	lines := []string{"These children would be left orphaned:"}
	for _, move := range q.Orphans {
		lines = append(lines, "  "+move.Branch+" would rebase onto "+move.To+" instead of "+move.From)
	}
	return strings.Join(lines, "\n")
}

func dropDataHelp(q clean.Questions) string {
	if len(q.Down) == 0 {
		return "Start a shared service that is down to drop the namespaces the removed worktrees made in it now, rather than once it next starts"
	}
	return "The removed worktrees made data in " + services(q) + ", which is down: start it to drop that data now, or keep it until it next starts (then run up or prune drops it)"
}

// services names the shared services down that the data waits on, "the service" when none is known.
func services(q clean.Questions) string {
	if len(q.Down) == 0 {
		return "the service"
	}
	return strings.Join(q.Down, ", ")
}

func forceHelp(q clean.Questions) string {
	if len(q.Unsafe) == 0 {
		return "Remove a worktree that is locked, has uncommitted changes, unpushed commits or an open pull request, or whose jobs would not stop"
	}

	lines := []string{"These worktrees are unsafe to remove:"}
	for _, removal := range q.Unsafe {
		lines = append(lines, "  "+removal.Worktree+" — "+output.Blockers(removal.Blockers))
	}
	return strings.Join(lines, "\n")
}
