package cli

import (
	"github.com/LucasPcq/wtm/next/internal/command/runlogs"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
)

// words are how the command line words each field of run logs' form.
func words(q runlogs.Questions, field domain.Field) form.Words {
	switch field.Spec().Path {
	case domain.PathWorktree:
		return form.Words{Flag: "<worktree>", Label: "Worktrees", Help: "The worktrees whose jobs to read, by branch, or by path on no branch (default: the one you stand in)", Empty: "Select at least one worktree", Options: form.RunningWords(q.Worktrees.Options)}
	case runlogs.PathJob:
		return form.Words{Flag: "--job", Label: "Job", Help: "Read this job alone, one of run.toml or one the run daemon runs there (default: every job that ran)"}
	case runlogs.PathFollow:
		return form.Words{Flag: "--follow", Shorthand: "f", Label: "Follow", Help: "Print each new line as the jobs print it, until they end or Ctrl+C"}
	case runlogs.PathSearch:
		return form.Words{Flag: "--search", Label: "Search", Help: "The lines holding this text in each job's whole log, its rotated files included (case ignored unless --match-case)"}
	case runlogs.PathRegex:
		return form.Words{Flag: "--regex", Label: "Regex", Help: "Read --search as a regular expression (RE2)"}
	case runlogs.PathMatchCase:
		return form.Words{Flag: "--match-case", Label: "Match case", Help: "Keep the case of --search"}
	case runlogs.PathSince:
		return form.Words{Flag: "--since", Label: "Since", Help: "The lines of each job's whole log printed from this time on (RFC 3339: 2026-11-03T14:00:00Z)"}
	}

	return form.Words{}
}
