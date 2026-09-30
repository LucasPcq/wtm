package up

import (
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/flow/run/concurrency"
	"github.com/LucasPcq/wtm/internal/flow/run/target"
	"github.com/LucasPcq/wtm/internal/rules"
)

func (f *upFlow) session() flow.Session {
	return flow.Session{
		ErrLabel: domain.CmdUp,
		Presets:  target.Presets(target.PresetParams{Worktrees: target.Dirs(f.named), Profile: f.request.Profile}),
		Steps: []flow.Step{
			target.WorktreesStep(target.WorktreesParams{
				ProjectDir: f.ctx.ProjectDir,
				Current:    f.request.Cwd,
				Selected:   target.Preselected(target.PreselectedParams{Named: f.named, Precheck: f.request.Precheck}),
				Running:    f.running,
				// --exclusive stops all but one, so it cannot be applied to a wider
				// selection. Refused as the box is ticked rather than after the recap.
				Single: f.request.Exclusive,
			}),
			target.ProfileStep(target.ProfileParams{Profiles: f.request.Config.Profiles}),
			f.concurrency.Step(),
		},
	}
}

func (f *upFlow) question() *concurrency.Question {
	return concurrency.New(concurrency.Params{
		Context:   f.ctx,
		Presenter: f.presenter,
		Exclusive: f.request.Exclusive,
		Parallel:  f.request.Parallel,
		Config:    f.request.Config,
		Running:   f.jobs,
		WorkDirs:  f.workDirs,
		Starting:  f.startingJobs,
	})
}

// startingJobs is empty while the profile cannot be resolved: the run refuses
// it later, and a question has nothing to measure in the meantime.
func (f *upFlow) startingJobs(answers flow.Answers) []domain.JobConfig {
	profile, err := f.resolveProfile(answers)
	if err != nil {
		return nil
	}
	return rules.JobsWithEffectivePorts(f.request.Config, profile.Jobs)
}

// workDirs are the worktrees this run acts on, as git spells them — the
// daemon's keys for every job it is about to start.
func (f *upFlow) workDirs(answers flow.Answers) []string {
	return target.WorkDirs(target.WorkDirsParams{Answers: answers, Named: f.named, Cwd: f.request.Cwd})
}
