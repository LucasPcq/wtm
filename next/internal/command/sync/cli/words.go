package cli

import (
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/command/sync"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
)

// words are how the command line words each field of sync's form; the descendants, force and parents name, on the questions observed, the worktrees and branches they are about.
func words(q sync.Questions, field domain.Field) form.Words {
	switch field.Spec().Path {
	case domain.PathWorktree:
		return form.Words{Flag: "<worktree>", Label: "Worktrees", Help: "The worktrees to sync, by branch or by path; those that need a sync are checked", Empty: "Select at least one worktree"}
	case sync.PathAll:
		return form.Words{Flag: "--all", Label: "All", Help: "Sync every worktree; one with uncommitted changes is left as it is unless --force", Options: map[string]form.OptionWords{
			"true":  {Label: "Every worktree", Short: "every worktree"},
			"false": {Label: "Those named", Short: "those named"},
		}}
	case sync.PathDescendants:
		them := "them"
		if count(q.Descendants.Under) == 1 {
			them = "it"
		}
		return form.Words{Flag: "--descendants", Label: "Descendants", Help: descendantsHelp(q.Descendants.Under), Options: map[string]form.OptionWords{
			"true":  {Label: "Yes, sync " + them + " too", Short: "with the worktrees under them"},
			"false": {Label: "No, only the worktrees asked", Short: "only those asked"},
		}}
	case sync.PathForce:
		return form.Words{Flag: "--force", Label: "Force", Help: forceHelp(q.Unsafe), Options: map[string]form.OptionWords{
			"true":  {Label: "Yes, rebase them anyway, their changes stashed around the rebase", Short: "forced"},
			"false": {Label: "No", Short: "not forced"},
		}}
	case sync.PathKeepConflict:
		return form.Words{Flag: "--keep-conflict", Label: "On conflict", Help: "Leave a rebase that stops on a conflict in progress, to resolve, instead of aborting it; the worktrees under it are skipped either way", Options: map[string]form.OptionWords{
			"true":  {Label: "Leave the rebase in progress, to resolve", Short: "leave it in progress"},
			"false": {Label: "Abort it, the worktree left as it was", Short: "abort it"},
		}}
	case sync.PathFFParents:
		return form.Words{Flag: "--ff-parents", Label: "Parents", Help: parentsHelp(q.Behind), Options: map[string]form.OptionWords{
			"true":  {Label: "Fast-forward them to origin first", Short: "fast-forwarded first"},
			"false": {Label: "Rebase onto them as they stand", Short: "as they stand"},
		}}
	case sync.PathPush:
		return form.Words{Flag: "--push", Label: "Push", Help: "Force-push each branch synced to origin, leasing on origin's branch as fetched", Options: map[string]form.OptionWords{
			"true":  {Label: "Yes, push them to origin", Short: "pushed"},
			"false": {Label: "No, keep them local", Short: "kept local"},
		}}
	case sync.PathNoFetch:
		return form.Words{Flag: "--no-fetch", Label: "No fetch", Help: "Read origin's branches as last fetched"}
	}

	return form.Words{}
}

// descendantsHelp asks whether the worktrees under those named come too, naming how many and under which; the schema's words when none is known.
func descendantsHelp(under map[string][]string) string {
	if len(under) == 0 {
		return "Sync the worktrees under those asked too"
	}

	named := slices.Sorted(maps.Keys(under))
	lines := make([]string, 0, len(named))
	for _, name := range named {
		lines = append(lines, "  "+strings.Join(under[name], ", ")+" under "+name)
	}
	n := count(under)
	question := "Sync the " + strconv.Itoa(n) + " worktrees under " + strings.Join(named, ", ") + " too?"
	if n == 1 {
		question = "Sync the worktree under " + strings.Join(named, ", ") + " too?"
	}
	return question + "\n" + strings.Join(lines, "\n")
}

func forceHelp(unsafe []sync.Rebase) string {
	if len(unsafe) == 0 {
		return "Rebase a worktree that has uncommitted changes, stashed around the rebase"
	}

	lines := []string{"These worktrees have uncommitted changes:"}
	for _, r := range unsafe {
		lines = append(lines, "  "+r.Worktree)
	}
	return strings.Join(lines, "\n")
}

func parentsHelp(behind []sync.Onto) string {
	if len(behind) == 0 {
		return "Fast-forward the branches rebased onto but not rebased, the base among them, to origin first"
	}

	lines := []string{"These branches are rebased onto and behind origin:"}
	for _, o := range behind {
		lines = append(lines, "  "+o.Branch+" ("+strconv.Itoa(o.Origin.Behind)+" behind)")
	}
	return strings.Join(lines, "\n")
}

// count is how many worktrees are under the roots of under, each once: subtrees of two roots may overlap.
func count(under map[string][]string) int {
	distinct := map[string]bool{}
	for _, below := range under {
		for _, name := range below {
			distinct[name] = true
		}
	}
	return len(distinct)
}
