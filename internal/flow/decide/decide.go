// Package decide holds the branch and env decisions the create-like flows share.
package decide

import (
	"context"
	"fmt"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/flow/envports"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/branch"
	"github.com/LucasPcq/wtm/internal/service/memory"
	"github.com/LucasPcq/wtm/internal/service/worktree"
)

func BranchCandidates(ctx context.Context, projectDir string) []domain.BranchCandidate {
	return branch.Candidates(ctx, branch.ListParams{ProjectDir: projectDir})
}

// MemoizedTarget caches branch classification for one run: it costs ~3 git
// subprocesses, and a step, a recap and a warning all classify the same name.
func MemoizedTarget(ctx context.Context, projectDir string) func(string) domain.BranchTarget {
	cache := make(map[string]domain.BranchTarget)
	return func(name string) domain.BranchTarget {
		if name == "" {
			return domain.BranchTarget{}
		}
		if target, ok := cache[name]; ok {
			return target
		}
		target := branch.Target(ctx, branch.BranchParams{ProjectDir: projectDir, Branch: name})
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
func SourceUpdate(ctx context.Context, params SourceUpdateParams) SourceUpdatePrompt {
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

	state, ab := branch.Divergence(ctx, branch.BranchParams{ProjectDir: params.ProjectDir, Branch: subject})
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
func EnvParentFallback(ctx context.Context, params EnvFallbackParams) (bool, string) {
	if params.Source == "" {
		return false, ""
	}
	applies := worktree.EnvParentFallsBackToMain(ctx, worktree.EnvFallbackParams{
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
	UpdateFastForward = domain.SourceUpdateFastForward
	UpdateKeep        = domain.SourceUpdateKeep
)

type SourceUpdateStepParams struct {
	Key         string
	Prompt      func(flow.Answers) SourceUpdatePrompt
	FastForward bool
}

// SourceUpdateStep applies only to a behind-only branch; a diverged one is not a
// gate here, it becomes a ⚠ line in the recap. --ff answers it through Resolve
// rather than as a preset, so under --ff it remembers nothing: the flag wins.
func SourceUpdateStep(params SourceUpdateStepParams) flow.Step {
	memory := flow.Memory{ID: domain.RememberSourceUpdate}
	if params.FastForward {
		memory = flow.Memory{}
	}
	return flow.Step{
		Memory: memory,
		Kind:   flow.StepSelect,
		Key:    params.Key,
		Label:  domain.SourceUpdateLabel,
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
func ApplyFastForward(ctx context.Context, params ApplyFastForwardParams) (proceed bool) {
	branchParams := branch.BranchParams{ProjectDir: params.ProjectDir, Branch: params.Subject}
	if !params.Prompter.Interactive() {
		_ = branch.FastForwardIfBehind(ctx, branchParams)
		return true
	}

	ffErr := params.Presenter.Stage(ctx, flow.StageParams{
		Message: fmt.Sprintf(domain.SourceFastForwardLoadingFmt, params.Subject),
		Work:    func(ctx context.Context) error { return branch.FastForwardToOrigin(ctx, branchParams) },
	})
	if ffErr == nil {
		return true
	}

	_, ab := branch.Divergence(ctx, branchParams)
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
func WarnUnseenFallback(ctx context.Context, params UnseenFallbackParams) []string {
	if params.Prompter.Interactive() {
		return nil
	}
	show, warning := EnvParentFallback(ctx, params.Fallback)
	if !show {
		return nil
	}
	params.Presenter.Status(flow.Notice{Kind: flow.NoticeWarning, Text: warning})
	return []string{warning}
}

type RememberParams struct {
	Context   flow.Context
	Session   flow.Session
	Answers   flow.Answers
	Presenter flow.Presenter
}

// Remember keeps what a confirmed session ticked. Failing to write it costs the
// next run a question, never this run its worktree.
func Remember(params RememberParams) {
	err := memory.Remember(memory.RememberParams{
		StateDir: params.Context.StateDir,
		Change:   flow.Remembering(params.Session, params.Answers),
	})
	if err != nil {
		params.Presenter.Status(flow.Notice{Kind: flow.NoticeWarning, Text: fmt.Sprintf(domain.RememberWriteFailedFmt, err)})
	}
}

// OriginsParams names the steps of a session creating a worktree, and the flags
// its request carried; an empty key is a question the session does not ask.
type OriginsParams struct {
	Context         flow.Context
	Answers         flow.Answers
	EnvKey          string
	IsolationKey    string
	SourceUpdateKey string
	EnvFlag         bool
	IsolationFlag   bool
	FastForward     bool
}

// Origins says, per memory id, what settled each answer a step could have
// remembered, for the JSON a script reads to know whether a remembered answer
// stood in for a flag it did not pass.
func Origins(params OriginsParams) map[string]domain.AnswerOrigin {
	questions := []struct {
		id       string
		key      string
		flag     bool
		fallback domain.AnswerOrigin
	}{
		{domain.RememberEnvStrategy, params.EnvKey, params.EnvFlag, domain.AnswerOriginConfig},
		{domain.RememberIsolation, params.IsolationKey, params.IsolationFlag, envports.IsolationOrigin(params.Context)},
		{domain.RememberSourceUpdate, params.SourceUpdateKey, params.FastForward, domain.AnswerOriginDefault},
	}
	origins := map[string]domain.AnswerOrigin{}
	for _, question := range questions {
		if question.key == "" {
			continue
		}
		origin, settled := flow.OriginOf(flow.OriginParams{
			Answers:  params.Answers,
			Key:      question.key,
			Flag:     question.flag,
			Fallback: question.fallback,
		})
		if settled {
			origins[question.id] = origin
		}
	}
	if len(origins) == 0 {
		return nil
	}
	return origins
}

// KeptSourceLines names a source left behind origin by a remembered or
// to-be-remembered "keep": asked, it is a visible choice; settled from memory,
// it would otherwise leave no trace in the recap.
func KeptSourceLines(answers flow.Answers, key string) []string {
	mark := flow.RememberedMark(answers, key)
	if mark == "" || answers.Value(key) != UpdateKeep {
		return nil
	}
	return []string{domain.RecapFieldSourceUpdate + domain.SourceUpdateSummaryKeep + mark}
}
