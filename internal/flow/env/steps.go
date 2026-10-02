package env

import (
	"fmt"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/rules"
)

const (
	KeyWorktree = "env.worktree"
	KeyAdopt    = "env.adopt"
	KeyResolve  = "env.resolve"
	KeyRecap    = "env.recap"
)

func (f *envFlow) session() flow.Session {
	return flow.Session{
		ErrLabel: domain.EnvWizardErrLabel,
		Presets:  flow.NewAnswers(map[string]string{KeyWorktree: f.request.Worktree}),
		Steps:    []flow.Step{f.worktreeStep(), f.adoptStep(), f.resolveStep(), f.recapStep()},
	}
}

// worktreeStep badges each worktree with its drift rather than its git state:
// the one thing the reader picks on here is which .env needs work.
func (f *envFlow) worktreeStep() flow.Step {
	options := make([]flow.Option, 0, len(f.statuses))
	for _, status := range f.statuses {
		var badges []flow.Badge
		if status.IsParent {
			badges = append(badges, flow.Badge{Text: domain.EnvBadgeParent})
		}
		scan := f.scans[status.Branch]
		badges = append(badges, driftBadge(scan.files))
		if scan.refused {
			badges = append(badges, flow.Badge{Text: fmt.Sprintf(domain.EnvBadgeRefusesFmt, f.request.Isolation), Tone: domain.ToneDanger})
		}
		options = append(options, flow.Option{Label: status.Branch, Value: status.Branch, Badges: badges, Disabled: scan.refused})
	}
	return flow.Step{
		Kind:    flow.StepSelect,
		Key:     KeyWorktree,
		Label:   domain.EnvWorktreeStepLabel,
		Title:   domain.EnvWorktreeStepTitle,
		Options: options,
		Arg:     true,
		Resolve: func(flow.Answers) (flow.Answer, error) {
			return flow.Answer{}, domain.ErrEnvWorktreeRequired
		},
	}
}

func driftBadge(files []domain.EnvFileResult) flow.Badge {
	count := rules.EnvDriftCount(files)
	if count == 0 {
		return flow.Badge{Text: domain.EnvBadgeInSync, Tone: domain.ToneSuccess}
	}
	return flow.Badge{Text: fmt.Sprintf(domain.EnvBadgeChangesFmt, count), Tone: domain.ToneWarning}
}

// adoptStep asks a worktree created before the isolation choice whether to
// adopt it. Keeping it as is opens under the cursor: adopting leaves its data
// behind in the compose project it runs under today.
func (f *envFlow) adoptStep() flow.Step {
	return flow.Step{
		Kind:  flow.StepSelect,
		Key:   KeyAdopt,
		Label: domain.IsolationAdoptStepName,
		Skip: func(answers flow.Answers) (bool, string) {
			return f.request.Isolation != "" || !f.scanOf(answers).adoption.Pending, ""
		},
		Build: func(answers flow.Answers) (flow.StepContent, error) {
			plan := f.scanOf(answers).adoption
			return flow.StepContent{
				Title:       fmt.Sprintf(domain.IsolationAdoptTitleFmt, answers.Value(KeyWorktree)),
				Description: domain.IsolationAdoptDescription,
				Options: []flow.Option{
					{Label: domain.IsolationAdoptKeepLabel, Value: domain.IsolationAdoptKeepValue},
					{Label: rules.IsolationAdoptOptionLabel(plan), Value: domain.IsolationAdoptValue, Danger: plan.ComposeProject != ""},
				},
			}, nil
		},
		Resolve: func(flow.Answers) (flow.Answer, error) {
			return flow.Answer{Value: domain.IsolationAdoptKeepValue}, nil
		},
		Summarize: func(answer flow.Answer) string {
			if answer.Value == domain.IsolationAdoptValue {
				return domain.IsolationAdoptSummary
			}
			return domain.IsolationAdoptKeptSummary
		},
	}
}

// resolveStep is skipped when there is nothing to decide: only safe additions,
// or a worktree in sync.
func (f *envFlow) resolveStep() flow.Step {
	return flow.Step{
		Kind:  flow.StepEnvResolve,
		Key:   KeyResolve,
		Label: domain.EnvResolveStepLabel,
		Skip: func(answers flow.Answers) (bool, string) {
			if rules.EnvResolvable(f.scanOf(answers).files) {
				return false, ""
			}
			return true, domain.EnvResolveSkipReason
		},
		Build: func(answers flow.Answers) (flow.StepContent, error) {
			return flow.StepContent{
				Title:    fmt.Sprintf(domain.EnvResolveTitleFmt, answers.Value(KeyWorktree)),
				EnvFiles: f.scanOf(answers).files,
				EnvDefaults: domain.EnvResolveDefaults{
					Overwrite: f.request.OnConflict == domain.EnvDecisionOverwrite,
					Prune:     f.request.Prune,
				},
			}, nil
		},
		Resolve: func(flow.Answers) (flow.Answer, error) {
			return flow.Answer{}, nil
		},
	}
}

func (f *envFlow) recapStep() flow.Step {
	return flow.Step{
		Kind:  flow.StepRecap,
		Key:   KeyRecap,
		Label: domain.EnvRecapStepLabel,
		Build: func(answers flow.Answers) (flow.StepContent, error) {
			options := f.applyOptions(answers)
			return flow.StepContent{
				Description: f.recap(recapParams{Answers: answers, VerbatimOffered: len(options) > 1}),
				Options:     options,
			}, nil
		},
		Resolve: func(flow.Answers) (flow.Answer, error) {
			return flow.Answer{Value: domain.EnvApplyValue}, nil
		},
	}
}

// applyOptions offers the port pass as a choice rather than a fait accompli. A
// worktree with no port to move keeps the single plain confirmation.
func (f *envFlow) applyOptions(answers flow.Answers) []flow.Option {
	apply := flow.Option{Label: domain.EnvApplyActionLabel, Value: domain.EnvApplyValue}
	if len(rules.EnvPortRewrites(f.scanOf(answers).ports)) == 0 {
		return []flow.Option{apply}
	}
	return []flow.Option{apply, {Label: domain.EnvApplyVerbatimLabel, Value: domain.EnvApplyVerbatimValue}}
}

type recapParams struct {
	Answers         flow.Answers
	VerbatimOffered bool
}

// recap restates the worktree, every decision with its value, and the port
// values the apply will shift — announced before it happens rather than
// discovered after.
func (f *envFlow) recap(params recapParams) string {
	answers := params.Answers
	branch := answers.Value(KeyWorktree)
	scan := f.scanOf(answers)
	lines := []string{
		domain.EnvRecapFieldWorktree + branch,
		domain.RecapFieldMode + string(f.request.Mode),
		domain.RecapFieldEnv + f.sourceLabel(branch),
	}
	if isolation := f.isolation(answers); isolation != "" {
		lines = append(lines, domain.RecapFieldIsolation+rules.IsolationSummary(isolation))
	}
	lines = append(lines, "")

	resolve, _ := answers.Get(KeyResolve)
	if body := rules.EnvResolveRecapLines(rules.EnvResolveRecapParams{Files: scan.files, Decisions: resolve.EnvDecisions}); len(body) > 0 && !resolve.Skipped {
		lines = append(lines, body...)
	} else {
		lines = append(lines, domain.EnvRecapSafeOnly)
	}
	lines = append(lines, rules.EnvPortRecapLines(scan.ports)...)
	lines = append(lines, rules.EnvRestoreRecapLines(rules.EnvRestoreRecapParams{
		Entries: scan.restore,
		Switch:  rules.IsVerbatim(f.request.Isolation),
		Offered: params.VerbatimOffered,
	})...)
	return strings.Join(lines, "\n")
}

// sourceLabel is where the values come from: the strategy, and the parent it
// reads when that strategy is "parent".
func (f *envFlow) sourceLabel(branch string) string {
	ctx := f.envContext(branch)
	if ctx.strategy == domain.EnvStrategyParent && ctx.parentBranch != "" {
		return string(ctx.strategy) + domain.EnvRecapNoteSeparator + ctx.parentBranch
	}
	return string(ctx.strategy)
}

func (f *envFlow) scanOf(answers flow.Answers) branchScan {
	return f.scans[answers.Value(KeyWorktree)]
}
