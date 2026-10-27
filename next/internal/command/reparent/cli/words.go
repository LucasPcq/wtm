package cli

import (
	"strconv"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/command/reparent"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
)

// words are how the command line words each field of reparent's form; each worktree reads with its parent, and the new parent says what moves and which branches are left out.
func words(q reparent.Questions, field domain.Field) form.Words {
	switch field.Spec().Path {
	case domain.PathWorktree:
		return form.Words{Flag: "<worktree>", Label: "Worktrees", Help: "The worktrees to move, by branch; the one you stand in by default", Empty: "Select at least one worktree", Options: worktreeWords(q.Worktrees.Options)}
	case reparent.PathTo:
		return form.Words{Flag: "--to", Label: "New parent", Help: toHelp(q)}
	}

	return form.Words{}
}

func worktreeWords(options []domain.Option) map[string]form.OptionWords {
	said := make(map[string]form.OptionWords, len(options))
	for _, option := range options {
		parent := "no parent"
		if option.Parent != "" {
			parent = "on " + option.Parent
		}
		said[option.Value] = form.OptionWords{Label: option.Value + "  " + parent}
	}

	return said
}

func toHelp(q reparent.Questions) string {
	var lines []string
	switch len(q.Moves) {
	case 0:
		return "The branch the worktrees are rebased onto from now on: a local branch, or origin/<branch>"
	case 1:
		lines = append(lines, movedFrom(q.Moves[0]))
	default:
		lines = append(lines, "New parent of "+strconv.Itoa(len(q.Moves))+" worktrees")
	}

	if len(q.To.Refused) > 0 {
		refused := make([]string, 0, len(q.To.Refused))
		for _, problem := range q.To.Refused {
			refused = append(refused, leftOut(problem))
		}
		lines = append(lines, "Left out: "+strings.Join(refused, ", "))
	}
	return strings.Join(lines, "\n")
}

// leftOut words a branch the new parent cannot be: one of the worktrees moved, or one under them.
func leftOut(problem domain.FieldError) string {
	if under := problem.Params["under"]; under != "" {
		return problem.Params["value"] + " (under " + under + ")"
	}

	return problem.Params["value"] + " (moved)"
}

func movedFrom(move reparent.Move) string {
	if move.From == "" {
		return move.Branch + " has no parent"
	}

	return move.Branch + " is rebased onto " + move.From
}
