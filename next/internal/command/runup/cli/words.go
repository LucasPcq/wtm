package cli

import (
	"strings"

	"github.com/LucasPcq/wtm/next/internal/command/runup"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
)

// words are how the command line words each field of run up's form.
func words(q runup.Questions, field domain.Field) form.Words {
	switch field.Spec().Path {
	case domain.PathWorktree:
		return form.Words{Flag: "<worktree>", Label: "Worktrees", Help: "The worktrees to start the profile in, by branch, or by path on no branch (default: the one you stand in)", Empty: "Select at least one worktree", Options: worktreeWords(q)}
	case runup.PathProfile:
		return form.Words{Flag: "--profile", Label: "Profile", Help: "The profile of run.toml to start (default: the one marked default, or the only one)", Options: profileWords(q)}
	case runup.PathExclusive:
		return form.Words{Flag: "--exclusive", Label: "Other worktrees", Help: "Stop the jobs the other worktrees run before starting (one worktree only)", Options: map[string]form.OptionWords{
			"false": {Label: "Keep them all running", Short: "kept running"},
			"true":  {Label: "Stop the other worktrees' jobs first", Short: "stopped first"},
		}}
	case runup.PathForce:
		return form.Words{Flag: "--force", Label: "Foreign data", Help: "Start a job whose touches reach data the worktree does not own (its source's, or a shared service's with no namespace)", Options: map[string]form.OptionWords{
			"false": {Label: "No, start nothing", Short: "no"},
			"true":  {Label: "Yes, start it anyway", Short: "forced"},
		}}
	case runup.PathParallel:
		return form.Words{Flag: "--parallel", Label: "Parallel", Help: "Keep the other worktrees' jobs running, whatever run.toml's concurrency says"}
	case runup.PathNoProbe:
		return form.Words{Flag: "--no-probe", Label: "No probe", Help: "Skip the check that each declared port answers"}
	case runup.PathDetach:
		return form.Words{Flag: "--detach", Shorthand: "d", Label: "Detach", Help: "Return once the jobs are started (the command line shows no live view yet)"}
	}

	return form.Words{}
}

// worktreeWords read each worktree with the jobs it runs, and the profile they were started under.
func worktreeWords(q runup.Questions) map[string]form.OptionWords {
	labels := map[string]form.OptionWords{}
	for _, option := range q.Worktrees.Options {
		if len(option.Up) == 0 {
			continue
		}
		running := strings.Join(option.Up, ", ")
		if option.Profile != "" {
			running = option.Profile + ": " + running
		}
		labels[option.Value] = form.OptionWords{Label: option.Value + " — runs " + running}
	}
	return labels
}

// profileWords read each profile with its jobs, in order, and whether it is the default.
func profileWords(q runup.Questions) map[string]form.OptionWords {
	labels := map[string]form.OptionWords{}
	for _, option := range q.Profiles {
		label := option.Value
		if len(option.Jobs) > 0 {
			label += " — " + strings.Join(option.Jobs, ", ")
		}
		if option.Default {
			label += " (default)"
		}
		labels[option.Value] = form.OptionWords{Label: label}
	}
	return labels
}
