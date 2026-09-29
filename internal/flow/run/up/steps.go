package up

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/flow/run/seam"
	"github.com/LucasPcq/wtm/internal/flow/run/target"
	"github.com/LucasPcq/wtm/internal/rules"
)

const KeyConcurrency = "run.concurrency"

// The four answers: what to do, and whether to be asked again. The "always"
// pair writes the choice to run.toml, which is the whole point of asking here
// rather than at every start.
// The plain answers are the domain values themselves, so what Resolve produces
// with no interaction and what the picker produces read back the same way.
const (
	answerParallel        = string(domain.ConcurrencyParallel)
	answerExclusive       = string(domain.ConcurrencyExclusive)
	answerParallelAlways  = answerParallel + alwaysSuffix
	answerExclusiveAlways = answerExclusive + alwaysSuffix
	alwaysSuffix          = "-always"
	// answerCancel is the clash declined: not starting is the one answer that
	// leaves the other worktree running.
	answerCancel = "cancel"
)

func (f *upFlow) session() flow.Session {
	return flow.Session{
		ErrLabel: domain.CmdUp,
		Presets:  target.Presets(target.PresetParams{Worktrees: target.Dirs(f.named), Profiles: f.request.Profiles}),
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
			target.ProfileStep(target.ProfileParams{
				Profiles: f.request.Config.Profiles,
				Default:  f.defaultProfile(),
			}),
			f.concurrencyStep(),
		},
	}
}

// concurrencyStep is asked at most once per project. It is skipped when nothing
// runs elsewhere — there is nothing to stop — and when run.toml already holds
// the answer.
func (f *upFlow) concurrencyStep() flow.Step {
	return flow.Step{
		Kind:  flow.StepSelect,
		Key:   KeyConcurrency,
		Label: domain.RunConcurrencyStepName,
		Skip: func(answers flow.Answers) (bool, string) {
			if f.decideConcurrency(answers).Ask {
				return false, ""
			}
			return true, f.skipReason(answers)
		},
		Build: func(answers flow.Answers) (flow.StepContent, error) {
			decision := f.decideConcurrency(answers)
			if decision.Clash {
				return f.clashContent(answers), nil
			}
			if decision.Contradiction {
				return f.contradictionContent(answers), nil
			}
			return flow.StepContent{
				Title:       domain.RunConcurrencyTitle,
				Description: fmt.Sprintf(domain.RunConcurrencyDescFmt, f.othersSummary(answers)),
				Options: []flow.Option{
					{Label: domain.RunConcurrencyParallel, Value: answerParallel},
					{Label: alwaysLabel(domain.RunConcurrencyParallel), Value: answerParallelAlways},
					{Separator: true},
					{Label: domain.RunConcurrencyExclusive, Value: answerExclusive},
					{Label: alwaysLabel(domain.RunConcurrencyExclusive), Value: answerExclusiveAlways},
				},
			}, nil
		},
		// Leaving the others alone is the answer that stops nothing, which is what
		// a safe default means here.
		Resolve: func(answers flow.Answers) (flow.Answer, error) {
			decision := f.decideConcurrency(answers)
			if decision.Clash {
				return flow.Answer{}, fmt.Errorf(domain.RunPortClashRefusedFmt,
					strings.Join(rules.PortClashLines(f.clashes(answers)), "\n"),
					domain.FlagExclusive, domain.FlagIsolation, domain.IsolationIsolated)
			}
			return flow.Answer{Value: string(decision.Value)}, nil
		},
		Summarize: func(answer flow.Answer) string { return string(concurrencyOf(answer.Value)) },
	}
}

// contradictionContent is the question a run contradicting the project's settled
// answer asks. Both ways out start every worktree that was selected: the run is
// what the user just asked for, and `exclusive` is what cannot be applied to it.
// Only the second one changes the setting — which is the whole reason this is a
// question rather than a notice.
func (f *upFlow) contradictionContent(answers flow.Answers) flow.StepContent {
	return flow.StepContent{
		Title: domain.RunConcurrencyContradictionTitle,
		Description: fmt.Sprintf(domain.RunConcurrencyContradictionDescFmt,
			f.request.Config.Concurrency, len(f.workDirs(answers))),
		Options: []flow.Option{
			{Label: domain.RunConcurrencyContradictionOnce, Value: answerParallel},
			{Label: domain.RunConcurrencyContradictionAlways, Value: answerParallelAlways},
		},
	}
}

// clashContent is the question when running side by side is not possible: a
// port this run needs is bound by another worktree.
func (f *upFlow) clashContent(answers flow.Answers) flow.StepContent {
	clashes := f.clashes(answers)
	names := make([]string, 0, len(clashes))
	for _, dir := range rules.ClashingWorktrees(clashes) {
		names = append(names, filepath.Base(dir))
	}
	return flow.StepContent{
		Title:       domain.RunPortClashTitle,
		Description: fmt.Sprintf(domain.RunPortClashDescFmt, strings.Join(rules.PortClashLines(clashes), "\n")),
		Options: []flow.Option{
			{Label: fmt.Sprintf(domain.RunPortClashStopFmt, strings.Join(names, domain.RunURLListSep)), Value: answerExclusive},
			{Label: domain.RunPortClashCancel, Value: answerCancel},
		},
	}
}

// skipReason says why the question was not put to anyone, which is not the same
// thing three times over: nothing to stop, a flag that already answered, or an
// answer this project settled for good.
func (f *upFlow) skipReason(answers flow.Answers) string {
	switch {
	case !f.othersRunning(answers):
		return domain.RunConcurrencySkipAlone
	case f.request.Exclusive || f.request.Parallel:
		return domain.RunConcurrencySkipFlag
	default:
		return domain.RunConcurrencySkipSettled
	}
}

// concurrency is what this run does about the other worktrees: the step's answer
// when it was actually put to someone, else whatever resolved it in its place.
// A skipped step carries no value — Skip short-circuits Resolve — so reading the
// answer alone would silently turn every non-interactive --exclusive into a
// parallel run.
func (f *upFlow) concurrency(answers flow.Answers) domain.Concurrency {
	if answers.Answered(KeyConcurrency) {
		return concurrencyOf(answers.Value(KeyConcurrency))
	}
	return f.decideConcurrency(answers).Value
}

func alwaysLabel(label string) string {
	return fmt.Sprintf(domain.RunConcurrencyAlwaysFmt, label)
}

// concurrencyOf reads any of the four answers, remembered or not.
func concurrencyOf(answer string) domain.Concurrency {
	if strings.TrimSuffix(answer, alwaysSuffix) == answerExclusive {
		return domain.ConcurrencyExclusive
	}
	return domain.ConcurrencyParallel
}

func remembers(answer string) bool { return strings.HasSuffix(answer, alwaysSuffix) }

func (f *upFlow) decideConcurrency(answers flow.Answers) rules.ConcurrencyDecision {
	return rules.DecideConcurrency(rules.ConcurrencyParams{
		Exclusive:     f.request.Exclusive,
		Parallel:      f.request.Parallel,
		Config:        f.request.Config.Concurrency,
		OthersRunning: f.othersRunning(answers),
		Selection:     len(f.workDirs(answers)),
		Clashes:       len(f.clashes(answers)) > 0,
	})
}

// clashes are the ports this run would bind that a job in a worktree it leaves
// alone already binds. Measured only when something runs elsewhere: it costs a
// git lookup per worktree involved, and with nothing up there is nothing to hit.
func (f *upFlow) clashes(answers flow.Answers) []domain.PortClash {
	if !f.othersRunning(answers) {
		return nil
	}
	return rules.PortClashes(rules.PortClashesParams{
		Starting: f.startingClaims(answers),
		Held:     f.heldClaims(answers),
	})
}

// startingClaims are the ports each selected worktree's jobs would bind.
func (f *upFlow) startingClaims(answers flow.Answers) []domain.PortClaim {
	profile, err := f.resolveProfile(answers)
	if err != nil {
		return nil
	}
	jobs := rules.JobsWithEffectivePorts(f.request.Config, profile.Jobs)
	var claims []domain.PortClaim
	for _, dir := range f.workDirs(answers) {
		offset, known := f.offsetOf(dir)
		if !known {
			continue
		}
		claims = append(claims, rules.PortClaims(rules.PortClaimsParams{Jobs: jobs, WorkDir: dir, Offset: offset})...)
	}
	return claims
}

// heldClaims are the ports the jobs up outside this run's selection bind, read
// from their declarations: the daemon keeps no port per job.
func (f *upFlow) heldClaims(answers flow.Answers) []domain.PortClaim {
	selected := make(map[string]bool)
	for _, dir := range f.workDirs(answers) {
		selected[dir] = true
	}
	declared := make(map[string]domain.JobConfig, len(f.request.Config.Jobs))
	for _, job := range rules.JobsWithEffectivePorts(f.request.Config, f.request.Config.Jobs) {
		declared[job.Name] = job
	}

	var claims []domain.PortClaim
	for _, info := range f.jobs {
		job, known := declared[info.Name]
		if !known || !rules.IsJobUp(info.Status) || selected[info.WorkDir] {
			continue
		}
		offset, resolved := f.offsetOf(info.WorkDir)
		if !resolved {
			continue
		}
		claims = append(claims, rules.PortClaims(rules.PortClaimsParams{
			Jobs:    []domain.JobConfig{job},
			WorkDir: info.WorkDir,
			Offset:  offset,
		})...)
	}
	return claims
}

// offsetOf is the offset a worktree runs on, read once per worktree: the steps
// ask again every time the wizard moves. A worktree whose offset cannot be
// resolved claims nothing rather than main's ports; the run refuses it anyway.
func (f *upFlow) offsetOf(dir string) (int, bool) {
	if offset, known := f.offsets[dir]; known {
		return offset, true
	}
	env, err := seam.JobEnv(seam.JobEnvParams{ProjectDir: f.ctx.ProjectDir, StateDir: f.ctx.StateDir, WorkDir: dir})
	if err != nil {
		return 0, false
	}
	offset := rules.PortOffsetFromEnv(env)
	if f.offsets == nil {
		f.offsets = map[string]int{}
	}
	f.offsets[dir] = offset
	return offset, true
}

// othersRunning is measured against the worktree the run targets, never against
// the current directory: `run up X` must not offer to stop X's own jobs.
func (f *upFlow) othersRunning(answers flow.Answers) bool {
	return len(f.otherWorktrees(answers)) > 0
}

// otherWorktrees is what runs outside this run's selection. Measured against the
// whole selection, never against one of them: `run up A B` must not offer to
// stop B's own jobs on A's behalf.
func (f *upFlow) otherWorktrees(answers flow.Answers) map[string][]string {
	selected := make(map[string]bool)
	for _, dir := range f.workDirs(answers) {
		selected[dir] = true
	}

	others := make(map[string][]string)
	for _, job := range f.jobs {
		if !rules.IsJobUp(job.Status) || selected[job.WorkDir] {
			continue
		}
		others[job.WorkDir] = append(others[job.WorkDir], job.Name)
	}
	return others
}

// othersSummary names the worktrees and their jobs, so the question says what
// it is about rather than that something is running somewhere.
func (f *upFlow) othersSummary(answers flow.Answers) string {
	others := f.otherWorktrees(answers)
	dirs := make([]string, 0, len(others))
	for dir := range others {
		dirs = append(dirs, dir)
	}
	sort.Strings(dirs)

	lines := make([]string, 0, len(dirs))
	for _, dir := range dirs {
		lines = append(lines, fmt.Sprintf("%s (%s)", filepath.Base(dir), strings.Join(others[dir], domain.RunURLListSep)))
	}
	return strings.Join(lines, domain.RunURLListSep)
}

// workDirs are the worktrees this run acts on, as git spells them — the
// daemon's keys for every job it is about to start.
func (f *upFlow) workDirs(answers flow.Answers) []string {
	return target.WorkDirs(target.WorkDirsParams{Answers: answers, Named: f.named, Cwd: f.request.Cwd})
}

func (f *upFlow) defaultProfile() string {
	profile, ok := rules.DefaultProfile(f.request.Config)
	if !ok {
		return ""
	}
	return profile.Name
}
