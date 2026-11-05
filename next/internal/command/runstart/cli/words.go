package cli

import (
	"github.com/LucasPcq/wtm/next/internal/command/runstart"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
)

// words are how the command line words each field of run start's form.
func words(q runstart.Questions, field domain.Field) form.Words {
	switch field.Spec().Path {
	case domain.PathWorktree:
		return form.Words{Flag: "<worktree>", Label: "Worktree", Help: "The worktree to start the job in, by branch, or by path on no branch (default: the one you stand in)", Options: form.RunningWords(q.Options)}
	case runstart.PathJob:
		return form.Words{Flag: "--job", Label: "Job", Help: "The job of run.toml to start", Options: form.JobWords(q.Jobs)}
	case runstart.PathRestart:
		return form.Words{Flag: "--restart", Label: "Restart", Help: "Stop the job and start it again when it is up (its log continues behind a [wtm] started line)"}
	case runstart.PathExclusive:
		return form.Words{Flag: "--exclusive", Label: "Other worktrees", Help: "Stop the jobs the other worktrees run before starting", Options: map[string]form.OptionWords{
			"false": {Label: "Keep them all running", Short: "kept running"},
			"true":  {Label: "Stop the other worktrees' jobs first", Short: "stopped first"},
		}}
	case runstart.PathForce:
		return form.Words{Flag: "--force", Label: "Foreign data", Help: "Start a job whose touches reach data the worktree does not own (its source's, or a shared service's with no namespace)", Options: map[string]form.OptionWords{
			"false": {Label: "No, start nothing", Short: "no"},
			"true":  {Label: "Yes, start it anyway", Short: "forced"},
		}}
	case runstart.PathParallel:
		return form.Words{Flag: "--parallel", Label: "Parallel", Help: "Keep the other worktrees' jobs running, whatever run.toml's concurrency says"}
	case runstart.PathNoProbe:
		return form.Words{Flag: "--no-probe", Label: "No probe", Help: "Skip the check that each declared port answers"}
	case runstart.PathDetach:
		return form.Words{Flag: "--detach", Shorthand: "d", Label: "Detach", Help: "Return once the job is started rather than show it live"}
	}

	return form.Words{}
}
