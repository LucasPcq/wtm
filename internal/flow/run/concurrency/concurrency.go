// Package concurrency asks what a start does about the jobs other worktrees
// are running, shared by `run up` and `run start`.
package concurrency

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/flow/run/seam"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/process"
	"github.com/LucasPcq/wtm/internal/service/runconfig"
)

const Key = "run.concurrency"

// The plain answers are the domain values themselves, so what Resolve produces
// with no interaction and what the picker produces read back the same way. The
// "always" pair writes the choice to run.toml.
const (
	answerParallel        = string(domain.ConcurrencyParallel)
	answerExclusive       = string(domain.ConcurrencyExclusive)
	answerParallelAlways  = answerParallel + alwaysSuffix
	answerExclusiveAlways = answerExclusive + alwaysSuffix
	alwaysSuffix          = "-always"
	answerCancel          = "cancel"
)

type Params struct {
	Context   flow.Context
	Presenter flow.Presenter
	// Exclusive and Parallel override the project's standing preference for one
	// run. They are the step's Resolve, not a second axis.
	Exclusive bool
	Parallel  bool
	Config    domain.RunConfig
	// Running is one reading of the daemon's index.
	Running []domain.JobInfo
	// WorkDirs are the worktrees the run acts on, as the answers so far settle them.
	WorkDirs func(flow.Answers) []string
	// Starting are the jobs the run would start, as the answers so far settle them.
	Starting func(flow.Answers) []domain.JobConfig
}

type Question struct {
	params Params
	// offsets memoizes each worktree's port offset: the steps ask again every
	// time the wizard moves.
	offsets map[string]int
}

func New(params Params) *Question {
	return &Question{params: params, offsets: map[string]int{}}
}

// Step is asked at most once per project. It is skipped when nothing runs
// elsewhere and when run.toml already holds the answer — unless a port this run
// needs is bound next door, where running side by side is no longer on offer.
func (q *Question) Step() flow.Step {
	return flow.Step{
		Kind:  flow.StepSelect,
		Key:   Key,
		Label: domain.RunConcurrencyStepName,
		Skip: func(answers flow.Answers) (bool, string) {
			if q.decide(answers).Ask {
				return false, ""
			}
			return true, q.skipReason(answers)
		},
		Build: func(answers flow.Answers) (flow.StepContent, error) {
			decision := q.decide(answers)
			if decision.Clash {
				return q.clashContent(answers), nil
			}
			if decision.Contradiction {
				return q.contradictionContent(answers), nil
			}
			return flow.StepContent{
				Title:       domain.RunConcurrencyTitle,
				Description: fmt.Sprintf(domain.RunConcurrencyDescFmt, q.othersSummary(answers)),
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
			decision := q.decide(answers)
			if decision.Clash {
				return flow.Answer{}, fmt.Errorf(domain.RunPortClashRefusedFmt,
					strings.Join(rules.PortClashLines(q.clashes(answers)), "\n"),
					domain.FlagExclusive, domain.FlagIsolation, domain.IsolationIsolated)
			}
			return flow.Answer{Value: string(decision.Value)}, nil
		},
		Summarize: func(answer flow.Answer) string { return string(concurrencyOf(answer.Value)) },
	}
}

// Cancelled is the clash declined: not starting is the one answer that leaves
// the other worktree running.
func (q *Question) Cancelled(answers flow.Answers) bool {
	return answers.Value(Key) == answerCancel
}

// Decided is what this run does about the other worktrees: the step's answer
// when it was actually put to someone, else whatever resolved it in its place.
// A skipped step carries no value — Skip short-circuits Resolve — so reading the
// answer alone would silently turn every non-interactive --exclusive into a
// parallel run.
func (q *Question) Decided(answers flow.Answers) domain.Concurrency {
	if answers.Answered(Key) {
		return concurrencyOf(answers.Value(Key))
	}
	return q.decide(answers).Value
}

// Apply carries the decision out, in the order a start needs it: the remembered
// answer written, the setting set aside said, the other worktrees stopped. It
// returns the run config the start goes on with.
func (q *Question) Apply(answers flow.Answers) (domain.RunConfig, error) {
	cfg, err := q.remember(answers)
	if err != nil {
		return cfg, err
	}
	q.noticeOverridden(answers)
	return cfg, q.clearOthers(answers)
}

// remember is never silent: a file changed without a word is a file nobody
// knows to change back.
func (q *Question) remember(answers flow.Answers) (domain.RunConfig, error) {
	answer := answers.Value(Key)
	if !remembers(answer) {
		return q.params.Config, nil
	}

	cfg := q.params.Config
	cfg.Concurrency = concurrencyOf(answer)
	if err := runconfig.Save(runconfig.SaveParams{StateDir: q.params.Context.StateDir, Config: cfg}); err != nil {
		return q.params.Config, fmt.Errorf("remember concurrency: %w", err)
	}
	q.params.Config = cfg
	q.params.Presenter.Status(flow.Notice{
		Kind: flow.NoticeMessage,
		Text: fmt.Sprintf(domain.RunConcurrencyRememberedFmt, cfg.Concurrency),
	})
	return cfg, nil
}

// noticeOverridden is only ever reached where nobody could be asked: the safe
// default destroys nothing, and a default that goes unsaid is a default nobody
// can correct.
func (q *Question) noticeOverridden(answers flow.Answers) {
	if answers.Answered(Key) || !q.decide(answers).Contradiction {
		return
	}
	q.params.Presenter.Status(warning(fmt.Sprintf(domain.RunConcurrencyOverriddenFmt,
		q.params.Config.Concurrency, len(q.params.WorkDirs(answers)))))
}

// clearOthers reports a worktree that refuses to stop and carries on: the
// answer was about this machine's load, not about a dependency.
func (q *Question) clearOthers(answers flow.Answers) error {
	if q.Decided(answers) != domain.ConcurrencyExclusive {
		return nil
	}
	others := q.otherWorktrees(answers)
	if len(others) == 0 {
		return nil
	}

	dirs := make([]string, 0, len(others))
	for dir := range others {
		dirs = append(dirs, dir)
	}
	sort.Strings(dirs)

	client := process.NewClient(process.SocketPath())
	// Reported after the stage, never inside it: a spinner owns the stream while
	// it runs, so a line written under it is repainted over.
	var reports []flow.Notice
	err := q.params.Presenter.Stage(flow.StageParams{
		Message: domain.RunStoppingOthers,
		Work: func() error {
			for _, dir := range dirs {
				reports = append(reports, stopReport(client, dir))
			}
			return nil
		},
	})
	if err != nil {
		return err
	}
	for _, report := range reports {
		q.params.Presenter.Status(report)
	}
	return nil
}

func stopReport(client *process.Client, dir string) flow.Notice {
	resp, err := client.Send(process.Request{Action: process.ActionStopAll, WorkDir: dir})
	if err != nil {
		return warning(fmt.Sprintf(domain.RunStopOtherFailFmt, filepath.Base(dir), err))
	}
	if resp.Status == process.StatusError {
		return warning(fmt.Sprintf(domain.RunStopOtherFailFmt, filepath.Base(dir), resp.Message))
	}
	return flow.Notice{
		Kind: flow.NoticeSuccess,
		Text: fmt.Sprintf(domain.RunStoppedOtherFmt, filepath.Base(dir)),
	}
}

func warning(text string) flow.Notice {
	return flow.Notice{Kind: flow.NoticeWarning, Text: text}
}

// contradictionContent is the question a run contradicting the project's settled
// answer asks. Both ways out start every worktree that was selected; only the
// second one changes the setting.
func (q *Question) contradictionContent(answers flow.Answers) flow.StepContent {
	return flow.StepContent{
		Title: domain.RunConcurrencyContradictionTitle,
		Description: fmt.Sprintf(domain.RunConcurrencyContradictionDescFmt,
			q.params.Config.Concurrency, len(q.params.WorkDirs(answers))),
		Options: []flow.Option{
			{Label: domain.RunConcurrencyContradictionOnce, Value: answerParallel},
			{Label: domain.RunConcurrencyContradictionAlways, Value: answerParallelAlways},
		},
	}
}

func (q *Question) clashContent(answers flow.Answers) flow.StepContent {
	clashes := q.clashes(answers)
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

func (q *Question) skipReason(answers flow.Answers) string {
	switch {
	case !q.othersRunning(answers):
		return domain.RunConcurrencySkipAlone
	case q.params.Exclusive || q.params.Parallel:
		return domain.RunConcurrencySkipFlag
	default:
		return domain.RunConcurrencySkipSettled
	}
}

func alwaysLabel(label string) string {
	return fmt.Sprintf(domain.RunConcurrencyAlwaysFmt, label)
}

func concurrencyOf(answer string) domain.Concurrency {
	if strings.TrimSuffix(answer, alwaysSuffix) == answerExclusive {
		return domain.ConcurrencyExclusive
	}
	return domain.ConcurrencyParallel
}

func remembers(answer string) bool { return strings.HasSuffix(answer, alwaysSuffix) }

func (q *Question) decide(answers flow.Answers) rules.ConcurrencyDecision {
	return rules.DecideConcurrency(rules.ConcurrencyParams{
		Exclusive:     q.params.Exclusive,
		Parallel:      q.params.Parallel,
		Config:        q.params.Config.Concurrency,
		OthersRunning: q.othersRunning(answers),
		Selection:     len(q.params.WorkDirs(answers)),
		Clashes:       len(q.clashes(answers)) > 0,
	})
}

// clashes are measured only when something runs elsewhere: it costs a git
// lookup per worktree involved, and with nothing up there is nothing to hit.
func (q *Question) clashes(answers flow.Answers) []domain.PortClash {
	if !q.othersRunning(answers) {
		return nil
	}
	return rules.PortClashes(rules.PortClashesParams{
		Starting: q.StartingClaims(answers),
		Held:     q.heldClaims(answers),
	})
}

// StartingClaims are the ports each selected worktree's jobs would bind.
func (q *Question) StartingClaims(answers flow.Answers) []domain.PortClaim {
	jobs := q.params.Starting(answers)
	var claims []domain.PortClaim
	for _, dir := range q.params.WorkDirs(answers) {
		offset, known := q.offsetOf(dir)
		if !known {
			continue
		}
		claims = append(claims, rules.PortClaims(rules.PortClaimsParams{Jobs: jobs, WorkDir: dir, Offset: offset})...)
	}
	return claims
}

// heldClaims are read from the declarations: the daemon keeps no port per job.
func (q *Question) heldClaims(answers flow.Answers) []domain.PortClaim {
	selected := selectedSet(q.params.WorkDirs(answers))
	declared := make(map[string]domain.JobConfig, len(q.params.Config.Jobs))
	for _, job := range rules.JobsWithEffectivePorts(q.params.Config, q.params.Config.Jobs) {
		declared[job.Name] = job
	}

	var claims []domain.PortClaim
	for _, info := range q.params.Running {
		job, known := declared[info.Name]
		if !known || !rules.IsJobUp(info.Status) || selected[info.WorkDir] {
			continue
		}
		offset, resolved := q.offsetOf(info.WorkDir)
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

// offsetOf leaves a worktree whose offset cannot be resolved claiming nothing
// rather than main's ports; the run refuses it anyway.
func (q *Question) offsetOf(dir string) (int, bool) {
	if offset, known := q.offsets[dir]; known {
		return offset, true
	}
	env, err := seam.JobEnv(seam.JobEnvParams{ProjectDir: q.params.Context.ProjectDir, StateDir: q.params.Context.StateDir, WorkDir: dir})
	if err != nil {
		return 0, false
	}
	offset := rules.PortOffsetFromEnv(env)
	q.offsets[dir] = offset
	return offset, true
}

func (q *Question) othersRunning(answers flow.Answers) bool {
	return len(q.otherWorktrees(answers)) > 0
}

// otherWorktrees is measured against the whole selection, never against the
// current directory: `run up A B` must not offer to stop B's own jobs on A's
// behalf.
func (q *Question) otherWorktrees(answers flow.Answers) map[string][]string {
	selected := selectedSet(q.params.WorkDirs(answers))
	others := make(map[string][]string)
	for _, job := range q.params.Running {
		if !rules.IsJobUp(job.Status) || selected[job.WorkDir] {
			continue
		}
		others[job.WorkDir] = append(others[job.WorkDir], job.Name)
	}
	return others
}

func selectedSet(dirs []string) map[string]bool {
	selected := make(map[string]bool, len(dirs))
	for _, dir := range dirs {
		selected[dir] = true
	}
	return selected
}

func (q *Question) othersSummary(answers flow.Answers) string {
	others := q.otherWorktrees(answers)
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
