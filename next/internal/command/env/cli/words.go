package cli

import (
	"strconv"
	"strings"

	envcmd "github.com/LucasPcq/wtm/next/internal/command/env"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
)

// words are how the command line words each field of env's form.
func words(q envcmd.Questions, field domain.Field) form.Words {
	switch field.Spec().Path {
	case domain.PathWorktree:
		return form.Words{Flag: "<worktree>", Label: "Worktrees", Help: "The worktrees to reconcile, by branch or by path; those out of step and the one you stand in are checked", Empty: "Select at least one worktree", Options: worktreeWords(q.Worktrees.Options)}
	case envcmd.PathAll:
		return form.Words{Flag: "--all", Label: "All", Help: "Reconcile every worktree", Options: map[string]form.OptionWords{
			"true":  {Label: "Every worktree", Short: "every worktree"},
			"false": {Label: "Those named", Short: "those named"},
		}}
	case envcmd.PathFrom:
		return form.Words{Flag: "--from", Label: "Values from", Help: "Where the values come from for this run: example, main or parent; each worktree's own strategy by default", Options: map[string]form.OptionWords{
			string(project.EnvExample): {Label: "The template", Short: "example"},
			string(project.EnvMain):    {Label: "The main checkout", Short: "main"},
			string(project.EnvParent):  {Label: "The parent worktree", Short: "parent"},
		}}
	case envcmd.PathOverwrite:
		return form.Words{Flag: "--overwrite", Label: "Overwrite", Help: keysHelp("Give a key whose value differs from its source the source's value", "These keys differ from their source:", q.Conflicts), Options: map[string]form.OptionWords{
			"true":  {Label: "Yes, take the source's value", Short: "overwritten"},
			"false": {Label: "No, keep the worktree's value", Short: "kept"},
		}}
	case envcmd.PathPrune:
		return form.Words{Flag: "--prune", Label: "Prune", Help: keysHelp("Drop a key no source has", "These keys are in no source:", q.Orphans), Options: map[string]form.OptionWords{
			"true":  {Label: "Yes, drop it", Short: "dropped"},
			"false": {Label: "No, keep it", Short: "kept"},
		}}
	case domain.PathIsolation:
		return form.Words{Flag: "--isolation", Label: "Isolation", Help: "Switch the worktrees to isolated (ports, compose project and namespaces of their own) or verbatim (the values wtm owns back to the source's); the one each has by default", Options: map[string]form.OptionWords{
			string(project.Isolated): {Label: "Isolated — ports, compose project and namespaces of its own", Short: "isolated"},
			string(project.Verbatim): {Label: "Verbatim — the values wtm owns back to the source's", Short: "verbatim"},
		}}
	case envcmd.PathShowValues:
		return form.Words{Flag: "--show-values", Label: "Show values", Help: "Show every value whole, secrets included; by default only what wtm writes is shown"}
	}

	return form.Words{}
}

// worktreeWords read each worktree option with what reconciling it changes, and its isolation when it is not isolated.
func worktreeWords(options []domain.Option) map[string]form.OptionWords {
	labels := map[string]form.OptionWords{}
	for _, option := range options {
		said := "in step"
		if option.OutOfStep > 0 {
			said = counted(option.OutOfStep)
		}
		if option.Isolation != "" {
			said = string(option.Isolation) + " · " + said
		}
		labels[option.Value] = form.OptionWords{Label: option.Value + " — " + said}
	}
	return labels
}

// keysHelp is help, or, when keys are asked about, title and each key.
func keysHelp(help, title string, keys []envcmd.KeyAt) string {
	if len(keys) == 0 {
		return help
	}
	lines := []string{title}
	for _, k := range keys {
		lines = append(lines, "  "+k.Worktree+"  "+k.File+"  "+k.Key)
	}
	return strings.Join(lines, "\n")
}

func counted(n int) string {
	if n == 1 {
		return "1 change"
	}
	return strconv.Itoa(n) + " changes"
}
