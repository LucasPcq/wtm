// Package decide holds the branch and env decisions the create-like flows share.
package decide

import (
	"fmt"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/branch"
	"github.com/LucasPcq/wtm/internal/service/worktree"
)

func BranchCandidates(projectDir string) []domain.BranchCandidate {
	return branch.Candidates(branch.ListParams{ProjectDir: projectDir})
}

// MemoizedTarget caches branch classification for one run: it costs ~3 git
// subprocesses, and a step, a recap and a warning all classify the same name.
func MemoizedTarget(projectDir string) func(string) domain.BranchTarget {
	cache := make(map[string]domain.BranchTarget)
	return func(name string) domain.BranchTarget {
		if name == "" {
			return domain.BranchTarget{}
		}
		if target, ok := cache[name]; ok {
			return target
		}
		target := branch.Target(branch.BranchParams{ProjectDir: projectDir, Branch: name})
		cache[name] = target
		return target
	}
}

type SourceUpdatePrompt struct {
	Branch      string
	Show        bool
	Title       string
	Description string
	Warning     string
	// AbortOnDecline distinguishes a diverged branch (declining cancels) from a
	// fast-forward offer (declining proceeds as-is).
	AbortOnDecline bool
	SkipReason     string
}

type SourceUpdateParams struct {
	ProjectDir string
	Target     func(string) domain.BranchTarget
	Branch     string
	Source     string
	Many       bool
}

// SourceUpdate classifies the divergence from origin of whichever branch the
// worktree starts from: the target branch when it already exists locally (its
// commits are what gets checked out), the source branch otherwise.
func SourceUpdate(params SourceUpdateParams) SourceUpdatePrompt {
	subject := params.Source
	if params.Target != nil && params.Branch != "" &&
		params.Target(params.Branch).State == domain.BranchTargetExisting {
		subject = params.Branch
	}
	if subject == "" {
		return SourceUpdatePrompt{SkipReason: domain.SourceUpdateSkipNoSource}
	}
	if rules.IsRemoteBranch(subject) {
		return SourceUpdatePrompt{Branch: subject, SkipReason: domain.SourceUpdateSkipRemote}
	}

	state, ab := branch.Divergence(branch.BranchParams{ProjectDir: params.ProjectDir, Branch: subject})
	if rules.ShouldOfferFastForward(state) {
		return SourceUpdatePrompt{
			Branch:      subject,
			Show:        true,
			Title:       fmt.Sprintf(domain.SourceFastForwardPrompt, subject, ab.Behind),
			Description: Pick(PickParams{Many: params.Many, One: domain.SourceFastForwardDescription, Several: domain.SourceFastForwardDescriptionMany}),
		}
	}
	if state == domain.DivergenceDiverged {
		return SourceUpdatePrompt{
			Branch:         subject,
			Show:           true,
			Title:          fmt.Sprintf(domain.SourceDivergedPrompt, subject, ab.Ahead, ab.Behind),
			Warning:        Pick(PickParams{Many: params.Many, One: domain.SourceDivergedWarning, Several: domain.SourceDivergedWarningMany}),
			AbortOnDecline: true,
			SkipReason:     domain.SourceUpdateSkipDiverged,
		}
	}
	return SourceUpdatePrompt{Branch: subject, SkipReason: domain.SourceUpdateSkipUpToDate}
}

type FastForwardSubjectParams struct {
	Target     domain.BranchTarget
	FromBranch string
	Branch     string
}

// FastForwardSubject names the branch a fast-forward updates: the source for a
// branch git is about to create, the worktree's own branch when it already exists.
func FastForwardSubject(params FastForwardSubjectParams) string {
	if rules.SourceIsStartPoint(params.Target.State) {
		return params.FromBranch
	}
	return params.Branch
}

type EnvFallbackParams struct {
	ProjectDir  string
	Source      string
	Config      domain.Config
	EnvOverride string
}

// EnvParentFallback: the "parent" strategy silently sources .env from the main
// worktree when the source branch has no local one.
func EnvParentFallback(params EnvFallbackParams) (bool, string) {
	if params.Source == "" {
		return false, ""
	}
	applies := worktree.EnvParentFallsBackToMain(worktree.EnvFallbackParams{
		ProjectDir:  params.ProjectDir,
		Source:      params.Source,
		Config:      params.Config,
		EnvOverride: params.EnvOverride,
	})
	if !applies {
		return false, ""
	}
	return true, domain.EnvParentFallbackWarning
}

type PickParams struct {
	Many    bool
	One     string
	Several string
}

func Pick(params PickParams) string {
	if params.Many {
		return params.Several
	}
	return params.One
}

// EnvOptions are the env strategies a create-like run offers, the config's own
// first and answered by the empty value.
func EnvOptions(strategy domain.EnvStrategy) []flow.Option {
	return []flow.Option{
		{Label: fmt.Sprintf(domain.EnvOptionConfigDefaultFmt, strategy), Value: ""},
		{Label: domain.EnvOptionExample, Value: string(domain.EnvStrategyExample)},
		{Label: domain.EnvOptionMain, Value: string(domain.EnvStrategyMain)},
		{Label: domain.EnvOptionParent, Value: string(domain.EnvStrategyParent)},
	}
}

type IsolationOptionsParams struct {
	First domain.Isolation
	Many  bool
}

func IsolationOptions(params IsolationOptionsParams) []flow.Option {
	label := rules.IsolationOptionLabel
	if params.Many {
		label = rules.IsolationOptionLabelMany
	}
	choices := rules.IsolationChoices(params.First)
	options := make([]flow.Option, 0, len(choices))
	for _, choice := range choices {
		options = append(options, flow.Option{Label: label(choice), Value: string(choice)})
	}
	return options
}

// UpdateFastForward and UpdateKeep answer a source-update step.
const (
	UpdateFastForward = "ff"
	UpdateKeep        = "keep"
)

type SourceUpdateStepParams struct {
	Key         string
	Prompt      func(flow.Answers) SourceUpdatePrompt
	FastForward bool
}

// SourceUpdateStep applies only to a behind-only branch; a diverged one is not a
// gate here, it becomes a ⚠ line in the recap.
func SourceUpdateStep(params SourceUpdateStepParams) flow.Step {
	return flow.Step{
		Kind:  flow.StepSelect,
		Key:   params.Key,
		Label: domain.SourceUpdateLabel,
		Skip: func(answers flow.Answers) (bool, string) {
			prompt := params.Prompt(answers)
			if prompt.Show && !prompt.AbortOnDecline {
				return false, ""
			}
			return true, prompt.SkipReason
		},
		Build: func(answers flow.Answers) (flow.StepContent, error) {
			prompt := params.Prompt(answers)
			return flow.StepContent{
				Description: prompt.Description,
				Options: []flow.Option{
					{Label: fmt.Sprintf(domain.SourceFastForwardOptionFmt, prompt.Branch), Value: UpdateFastForward},
					{Separator: true},
					{Label: domain.SourceKeepAsIsOption, Value: UpdateKeep},
				},
			}, nil
		},
		Resolve: func(flow.Answers) (flow.Answer, error) {
			if params.FastForward {
				return flow.Answer{Value: UpdateFastForward}, nil
			}
			return flow.Answer{Value: UpdateKeep}, nil
		},
		Summarize: func(answer flow.Answer) string {
			if answer.Value == UpdateFastForward {
				return domain.SourceUpdateSummaryFastForward
			}
			return domain.SourceUpdateSummaryKeep
		},
		Flag: domain.FlagFF,
	}
}

type ApplyFastForwardParams struct {
	ProjectDir string
	Subject    string
	Many       bool
	Prompter   flow.Prompter
	Presenter  flow.Presenter
}

// ApplyFastForward runs an accepted fast-forward. Unattended (--ff) it is best
// effort: a branch that cannot be cleanly fast-forwarded is left as-is and the
// run proceeds from it. Interactively a failure asks whether to go on from the
// stale branch; proceed is false when that is declined.
func ApplyFastForward(params ApplyFastForwardParams) (proceed bool) {
	branchParams := branch.BranchParams{ProjectDir: params.ProjectDir, Branch: params.Subject}
	if !params.Prompter.Interactive() {
		_ = branch.FastForwardIfBehind(branchParams)
		return true
	}

	ffErr := params.Presenter.Stage(flow.StageParams{
		Message: fmt.Sprintf(domain.SourceFastForwardLoadingFmt, params.Subject),
		Work:    func() error { return branch.FastForwardToOrigin(branchParams) },
	})
	if ffErr == nil {
		return true
	}

	_, ab := branch.Divergence(branchParams)
	proceed, err := params.Prompter.Confirm(flow.ConfirmParams{
		Title:      fmt.Sprintf(Pick(PickParams{Many: params.Many, One: domain.SourceProceedStalePrompt, Several: domain.SourceProceedStalePromptMany}), params.Subject, ab.Behind),
		Warning:    fmt.Sprintf(domain.SourceProceedStaleWarning, ffErr),
		DefaultYes: false,
	})
	return err == nil && proceed
}

type UnseenFallbackParams struct {
	Fallback  EnvFallbackParams
	Prompter  flow.Prompter
	Presenter flow.Presenter
}

// WarnUnseenFallback says after the fact what a recap warns an interactive run
// about: an unattended one never saw it, and its .env came from main rather
// than from the parent it named.
func WarnUnseenFallback(params UnseenFallbackParams) []string {
	if params.Prompter.Interactive() {
		return nil
	}
	show, warning := EnvParentFallback(params.Fallback)
	if !show {
		return nil
	}
	params.Presenter.Status(flow.Notice{Kind: flow.NoticeWarning, Text: warning})
	return []string{warning}
}
