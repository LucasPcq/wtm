package checkout

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/flow/decide"
	"github.com/LucasPcq/wtm/internal/flow/envports"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/branch"
	ghservice "github.com/LucasPcq/wtm/internal/service/github"
	"github.com/LucasPcq/wtm/internal/service/worktree"
)

const (
	KeyPR           = "checkout.pr"
	KeyParent       = "checkout.parent"
	KeyEnv          = "checkout.env"
	KeyIsolation    = "checkout.isolation"
	KeySourceUpdate = "checkout.source_update"
	KeyRecap        = "checkout.recap"
)

const confirmCheckout = "checkout"

func (f *checkoutFlow) session(ctx context.Context) flow.Session {
	return flow.Recall(flow.RecallParams{Session: f.steps(ctx), Remembered: f.ctx.Config.Project.Wizard.Remembered, Ask: f.request.Ask})
}

func (f *checkoutFlow) steps(ctx context.Context) flow.Session {
	return flow.Session{
		ErrLabel: domain.WizardErrLabel,
		Presets:  flow.NewAnswers(f.presets()),
		Steps: []flow.Step{
			f.prStep(ctx),
			f.parentStep(),
			f.envStep(),
			f.isolationStep(),
			f.sourceUpdateStep(),
			f.recapStep(),
		},
	}
}

func (f *checkoutFlow) presets() map[string]string {
	presets := map[string]string{
		KeyParent:    f.request.From,
		KeyEnv:       f.request.EnvFrom,
		KeyIsolation: string(f.request.Isolation),
	}
	if f.request.Number > 0 {
		presets[KeyPR] = strconv.Itoa(f.request.Number)
	}
	return presets
}

func (f *checkoutFlow) pr(answers flow.Answers) (domain.PRInfo, bool) {
	number, err := strconv.Atoi(answers.Value(KeyPR))
	if err != nil {
		return domain.PRInfo{}, false
	}
	return f.findPR(number)
}

func (f *checkoutFlow) prStep(ctx context.Context) flow.Step {
	return flow.Step{
		Kind:           flow.StepSelect,
		Key:            KeyPR,
		Label:          domain.CheckoutPRLabel,
		Title:          domain.CheckoutPRTitle,
		Description:    domain.CheckoutPRDescription,
		LoadingMessage: domain.LoadingPRsText,
		Load:           func(flow.Answers) (flow.StepContent, error) { return f.loadPRs(ctx), nil },
		Resolve: func(flow.Answers) (flow.Answer, error) {
			return flow.Answer{}, errors.New(domain.CheckoutPRRequired)
		},
		Summarize: func(answer flow.Answer) string {
			if answer.Value == "" {
				return ""
			}
			return "#" + answer.Value
		},
		Arg: true,
	}
}

// loadPRs disables the pull requests that cannot be checked out here: one whose
// branch a worktree already holds, and one from a fork.
func (f *checkoutFlow) loadPRs(ctx context.Context) flow.StepContent {
	prs, conn := ghservice.ListOpenPRsWithConnection(ctx, ghservice.ListPRsParams{ProjectDir: f.ctx.ProjectDir, Filter: f.request.Filter})
	f.setPRs(prs)

	linked := map[string]bool{}
	if worktrees, err := worktree.ListAll(f.runCtx, worktree.ListAllParams{ProjectDir: f.ctx.ProjectDir}); err == nil {
		for _, wt := range worktrees {
			linked[wt.Branch] = true
		}
	}

	options := make([]flow.Option, 0, len(prs))
	for _, pr := range prs {
		option := flow.Option{Label: rules.CheckoutPRLabel(pr), Value: strconv.Itoa(pr.Number)}
		switch {
		case linked[pr.Branch]:
			option.Disabled = true
			option.Badges = []flow.Badge{{Text: domain.BadgeTextLinked, Tone: domain.ToneNeutral}}
		case pr.IsFork:
			option.Disabled = true
			option.Badges = []flow.Badge{{Text: domain.BadgeTextFork, Tone: domain.ToneWarning}}
		}
		options = append(options, option)
	}

	title, lines := rules.GHConnectionBanner(conn)
	if title == "" && len(prs) == 0 {
		title = domain.CheckoutNoPRs
	}
	return flow.StepContent{Options: options, Banner: flow.Banner{Title: title, Lines: lines}}
}

func (f *checkoutFlow) base(answers flow.Answers) string {
	pr, _ := f.pr(answers)
	return pr.BaseBranch
}

func (f *checkoutFlow) parentStep() flow.Step {
	return flow.Step{
		Kind:         flow.StepBranchSelect,
		Key:          KeyParent,
		Label:        domain.CheckoutParentLabel,
		Title:        domain.CheckoutParentLabel,
		Description:  domain.CheckoutParentDescription,
		Branches:     f.candidates,
		PinnedSuffix: domain.PinnedSuffixBase,
		PinAbsent:    true,
		Refresh: func() []domain.BranchCandidate {
			return branch.Refresh(f.runCtx, branch.ListParams{ProjectDir: f.ctx.ProjectDir})
		},
		Build: func(answers flow.Answers) (flow.StepContent, error) {
			return flow.StepContent{Pinned: f.base(answers)}, nil
		},
		Resolve: func(answers flow.Answers) (flow.Answer, error) {
			return flow.Answer{Value: f.base(answers)}, nil
		},
		Flag: domain.FlagFrom,
	}
}

func (f *checkoutFlow) envStep() flow.Step {
	return flow.Step{
		Kind:        flow.StepSelect,
		Key:         KeyEnv,
		Label:       domain.CheckoutEnvLabel,
		Title:       domain.CheckoutEnvLabel,
		Description: domain.CreateEnvStepDescription,
		Options:     decide.EnvOptions(f.ctx.Config.Project.Env.Strategy),
		Resolve:     func(flow.Answers) (flow.Answer, error) { return flow.Answer{Value: ""}, nil },
		Summarize:   envSummary,
		Flag:        domain.FlagEnvFrom,
		Memory:      flow.Memory{ID: domain.RememberEnvStrategy},
	}
}

func envSummary(answer flow.Answer) string {
	if answer.Value != "" {
		return answer.Value
	}
	return domain.EnvSummaryConfigDefault
}

func (f *checkoutFlow) isolationStep() flow.Step {
	fallback := envports.DefaultIsolation(f.ctx)
	return flow.Step{
		Kind:        flow.StepSelect,
		Key:         KeyIsolation,
		Label:       domain.IsolationStepName,
		Title:       domain.IsolationStepName,
		Description: domain.IsolationStepDescription,
		Skip: func(flow.Answers) (bool, string) {
			if f.applies {
				return false, ""
			}
			return true, domain.IsolationStepIrrelevant
		},
		Options:   decide.IsolationOptions(decide.IsolationOptionsParams{First: fallback}),
		Resolve:   func(flow.Answers) (flow.Answer, error) { return flow.Answer{Value: string(fallback)}, nil },
		Summarize: func(answer flow.Answer) string { return rules.IsolationSummary(domain.Isolation(answer.Value)) },
		Flag:      domain.FlagIsolation,
		Memory:    flow.Memory{ID: domain.RememberIsolation},
	}
}

// sourceUpdateStep offers to fast-forward the PR's branch only when it exists
// here: a new one starts from origin, so there is nothing behind to update.
func (f *checkoutFlow) sourceUpdateStep() flow.Step {
	return decide.SourceUpdateStep(decide.SourceUpdateStepParams{
		Key:         KeySourceUpdate,
		Prompt:      f.sourceUpdate,
		FastForward: f.request.FastForward,
	})
}

func (f *checkoutFlow) sourceUpdate(answers flow.Answers) decide.SourceUpdatePrompt {
	pr, found := f.pr(answers)
	if !found || f.target(pr.Branch).State != domain.BranchTargetExisting {
		return decide.SourceUpdatePrompt{SkipReason: domain.CheckoutSourceUpdateSkipNew}
	}
	return decide.SourceUpdate(f.runCtx, decide.SourceUpdateParams{
		ProjectDir: f.ctx.ProjectDir,
		Target:     f.target,
		Branch:     pr.Branch,
		Source:     pr.Branch,
	})
}

func (f *checkoutFlow) recapStep() flow.Step {
	return flow.Step{
		Kind:  flow.StepRecap,
		Key:   KeyRecap,
		Label: domain.CheckoutRecapLabel,
		Build: func(answers flow.Answers) (flow.StepContent, error) {
			return flow.StepContent{
				Description: f.recap(answers),
				Options:     []flow.Option{{Label: domain.CheckoutRecapConfirmOption, Value: confirmCheckout}},
			}, nil
		},
		Resolve: func(flow.Answers) (flow.Answer, error) {
			return flow.Answer{Value: confirmCheckout}, nil
		},
	}
}

// recap names, beyond the answers, the one thing a user confirming "checkout
// PR #42" would not otherwise know: that its branch exists here and is reused.
func (f *checkoutFlow) recap(answers flow.Answers) string {
	var lines []string
	pr, found := f.pr(answers)
	if found {
		lines = append(lines, domain.RecapFieldPR+fmt.Sprintf(domain.CheckoutPRRecapFmt, pr.Number, pr.Title))
		if f.target(pr.Branch).State == domain.BranchTargetExisting {
			lines = append(lines, domain.RecapFieldBranch+pr.Branch+domain.BranchReusedSuffix)
		}
	}
	source := rules.FirstNonEmpty(answers.Value(KeyParent), pr.BaseBranch)
	if source != "" {
		lines = append(lines, domain.RecapFieldParent+source)
	}
	env := answers.Value(KeyEnv)
	lines = append(lines, domain.RecapFieldEnv+envSummary(flow.Answer{Value: env})+flow.RememberedMark(answers, KeyEnv))
	if rules.IsolationRecapShown(rules.IsolationRecapShownParams{Applies: f.applies, Override: f.request.Isolation}) {
		lines = append(lines, domain.RecapFieldIsolation+rules.IsolationSummary(f.isolation(answers))+flow.RememberedMark(answers, KeyIsolation))
	}
	if answers.Value(KeySourceUpdate) == decide.UpdateFastForward {
		lines = append(lines, fmt.Sprintf(domain.RecapUpdateFastForward, pr.Branch)+flow.RememberedMark(answers, KeySourceUpdate))
	}
	lines = append(lines, decide.KeptSourceLines(answers, KeySourceUpdate)...)
	lines = append(lines, flow.RememberedHint(answers, KeyEnv, KeyIsolation, KeySourceUpdate)...)

	var warnings []string
	if prompt := f.sourceUpdate(answers); prompt.Show && prompt.AbortOnDecline && prompt.Warning != "" {
		warnings = append(warnings, domain.WarningPrefix+prompt.Warning)
	}
	if show, warning := decide.EnvParentFallback(f.runCtx, decide.EnvFallbackParams{
		ProjectDir:  f.ctx.ProjectDir,
		Source:      source,
		Config:      f.ctx.Config,
		EnvOverride: env,
	}); show {
		warnings = append(warnings, domain.WarningPrefix+warning)
	}
	if len(warnings) > 0 {
		lines = append(lines, "")
		lines = append(lines, warnings...)
	}
	return strings.Join(lines, "\n")
}
