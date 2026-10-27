package cli

import (
	"github.com/LucasPcq/wtm/next/internal/command/exec"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
)

// words are how the command line words each field of exec's form.
func words(_ exec.Questions, field domain.Field) form.Words {
	switch field.Spec().Path {
	case domain.PathWorktree:
		return form.Words{Flag: "<worktree>", Label: "Worktrees", Help: "The worktrees to run the command in, by branch, or by path on no branch", Empty: "Select at least one worktree"}
	case exec.PathCommand:
		return form.Words{Flag: "-- <command>", Label: "Command", Help: "A /bin/sh line, run from each worktree's root"}
	case exec.PathAll:
		return form.Words{Flag: "--all", Label: "All", Help: "Run in every worktree, the main one included"}
	case exec.PathJobs:
		return form.Words{Flag: "--jobs", Label: "Concurrency", Help: "How many worktrees run the command at once (default: one per CPU)"}
	case exec.PathPrint:
		return form.Words{Flag: "--print", Label: "Print", Help: "Print each worktree's whole output, successes included"}
	}

	return form.Words{}
}
