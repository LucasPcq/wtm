package create

import (
	"errors"
	"fmt"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/flow/decide"
	"github.com/LucasPcq/wtm/internal/flow/envports"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/branch"
)

const (
	KeyBranch       = "create.branch"
	KeySource       = "create.source"
	KeyEnv          = "create.env"
	KeyIsolation    = "create.isolation"
	KeySourceUpdate = "create.source_update"
	KeyRecap        = "create.recap"
)

const (
	updateFastForward = "ff"
	updateKeep        = "keep"
	confirmCreate     = "create"
)

const (
	labelBranch       = "Branch name"
	labelSource       = "Source branch"
	labelEnv          = "Env strategy"
	labelSourceUpdate = "Source update"
	labelRecap        = "Confirm & create"
)

// TEMPORARY DUPLICATION: these steps are also declared, in Bubbletea terms, by
// internal/tui/newwt. `wtm extract` embeds that version as a sub-flow of its own
// combined wizard, so it cannot go until extract migrates here too (LUC-173, lot 4).
// Until then both must change together: same steps, same prose, same recap.
func (f *createFlow) session() flow.Session {
	return flow.Session{
		ErrLabel: domain.WizardErrLabel,
		Presets: flow.NewAnswers(map[string]string{
			KeyBranch:    f.request.Branch,
			KeySource:    f.request.From,
			KeyEnv:       f.request.EnvFrom,
			KeyIsolation: string(f.request.Isolation),
		}),
		Steps: []flow.Step{
			f.branchStep(),
			f.sourceStep(),
			f.envStep(),
			f.isolationStep(),
			f.sourceUpdateStep(),
			f.recapStep(),
		},
	}
}

func (f *createFlow) branchStep() flow.Step {
	return flow.Step{
		Kind:        flow.StepText,
		Key:         KeyBranch,
		Label:       labelBranch,
		Title:       labelBranch,
		Description: domain.CreateBranchStepDescription,
		Validate: func(value string) error {
			if strings.TrimSpace(value) == "" {
				return errors.New(domain.CreateBranchRequired)
			}
			return nil
		},
		Resolve: func(flow.Answers) (flow.Answer, error) {
			return flow.Answer{}, errors.New(domain.CreateBranchRequiredUnattended)
		},
	}
}

func (f *createFlow) sourceStep() flow.Step {
	return flow.Step{
		Kind:        flow.StepBranchSelect,
		Key:         KeySource,
		Label:       labelSource,
		Title:       labelSource,
		Description: domain.CreateSourceStepDescription,
		Branches:    f.candidates,
		Pinned:      f.ctx.Config.Project.Worktrees.BaseBranch,
		Refresh: func() []domain.BranchCandidate {
			return branch.Refresh(branch.ListParams{ProjectDir: f.ctx.ProjectDir})
		},
		Build: func(answers flow.Answers) (flow.StepContent, error) {
			description := domain.CreateSourceStepDescription
			if f.reusesBranch(answers) {
				description = domain.RecapParentRecordedForSync
			}
			return flow.StepContent{Title: labelSource, Description: description}, nil
		},
		Resolve: f.resolveSource,
		Flag:    domain.FlagFrom,
	}
}

// resolveSource refuses to guess the parent of a branch that already exists: it was
// created outside wtm, so defaulting would record a guess that `sync` and `tree`
// then treat as fact.
func (f *createFlow) resolveSource(answers flow.Answers) (flow.Answer, error) {
	branchName := answers.Value(KeyBranch)
	if rules.ParentMustBeExplicit(f.target(branchName).State) {
		return flow.Answer{}, fmt.Errorf(domain.ParentRequiredFmt, branchName, domain.FlagFrom)
	}
	base := f.ctx.Config.Project.Worktrees.BaseBranch
	if base == "" {
		return flow.Answer{}, fmt.Errorf(domain.CreateNoSourceFmt, domain.FlagFrom)
	}
	if !rules.BranchCandidateExists(f.candidates, base) {
		return flow.Answer{}, fmt.Errorf("%w: %s", domain.ErrBranchNotFound, base)
	}
	return flow.Answer{Value: base}, nil
}

func (f *createFlow) envStep() flow.Step {
	strategy := string(f.ctx.Config.Project.Env.Strategy)
	return flow.Step{
		Kind:        flow.StepSelect,
		Key:         KeyEnv,
		Label:       labelEnv,
		Title:       labelEnv,
		Description: domain.CreateEnvStepDescription,
		Options: []flow.Option{
			{Label: fmt.Sprintf(domain.EnvOptionConfigDefaultFmt, strategy), Value: ""},
			{Label: domain.EnvOptionExample, Value: string(domain.EnvStrategyExample)},
			{Label: domain.EnvOptionMain, Value: string(domain.EnvStrategyMain)},
			{Label: domain.EnvOptionParent, Value: string(domain.EnvStrategyParent)},
		},
		Resolve:   func(flow.Answers) (flow.Answer, error) { return flow.Answer{Value: ""}, nil },
		Summarize: envSummary,
		Flag:      domain.FlagEnvFrom,
	}
}

func envSummary(answer flow.Answer) string {
	if answer.Value != "" {
		return answer.Value
	}
	return domain.EnvSummaryConfigDefault
}

// isolationStep is asked here rather than after the worktree exists: what it
// decides is written into the .env this very run provisions, so it is one
// confirmation among the others instead of a second one past the point of no
// return.
func (f *createFlow) isolationStep() flow.Step {
	// Read once, as the session is built: Skip is called again on every step the
	// wizard advances through or steps back over, and run.toml does not change
	// under a run that is being answered.
	applies := envports.IsolationApplies(f.ctx)
	fallback := envports.DefaultIsolation(f.ctx)
	return flow.Step{
		Kind:        flow.StepSelect,
		Key:         KeyIsolation,
		Label:       domain.IsolationStepName,
		Title:       domain.IsolationStepName,
		Description: domain.IsolationStepDescription,
		Skip: func(flow.Answers) (bool, string) {
			if applies {
				return false, ""
			}
			return true, domain.IsolationStepIrrelevant
		},
		Options: isolationOptions(fallback),
		Resolve: func(flow.Answers) (flow.Answer, error) {
			return flow.Answer{Value: string(fallback)}, nil
		},
		Summarize: func(answer flow.Answer) string { return rules.IsolationSummary(domain.Isolation(answer.Value)) },
		Flag:      domain.FlagIsolation,
	}
}

func isolationOptions(first domain.Isolation) []flow.Option {
	choices := rules.IsolationChoices(first)
	options := make([]flow.Option, 0, len(choices))
	for _, choice := range choices {
		options = append(options, flow.Option{Label: rules.IsolationOptionLabel(choice), Value: string(choice)})
	}
	return options
}

// sourceUpdateStep applies only to a behind-only branch; a diverged one is not a
// gate here, it becomes a ⚠ line in the recap.
func (f *createFlow) sourceUpdateStep() flow.Step {
	return flow.Step{
		Kind:  flow.StepSelect,
		Key:   KeySourceUpdate,
		Label: labelSourceUpdate,
		Skip: func(answers flow.Answers) (bool, string) {
			prompt := f.sourceUpdate(answers)
			if prompt.Show && !prompt.AbortOnDecline {
				return false, ""
			}
			return true, prompt.SkipReason
		},
		Build: func(answers flow.Answers) (flow.StepContent, error) {
			prompt := f.sourceUpdate(answers)
			return flow.StepContent{
				Description: prompt.Description,
				Options: []flow.Option{
					{Label: fmt.Sprintf(domain.SourceFastForwardOptionFmt, prompt.Branch), Value: updateFastForward},
					{Separator: true},
					{Label: domain.SourceKeepAsIsOption, Value: updateKeep},
				},
			}, nil
		},
		Resolve: func(flow.Answers) (flow.Answer, error) {
			if f.request.FastForward {
				return flow.Answer{Value: updateFastForward}, nil
			}
			return flow.Answer{Value: updateKeep}, nil
		},
		Summarize: func(answer flow.Answer) string {
			if answer.Value == updateFastForward {
				return domain.SourceUpdateSummaryFastForward
			}
			return domain.SourceUpdateSummaryKeep
		},
		Flag: domain.FlagFF,
	}
}

func (f *createFlow) recapStep() flow.Step {
	return flow.Step{
		Kind:  flow.StepRecap,
		Key:   KeyRecap,
		Label: labelRecap,
		Build: func(answers flow.Answers) (flow.StepContent, error) {
			return flow.StepContent{
				Description: f.recap(answers),
				Options:     []flow.Option{{Label: domain.CreateRecapConfirmOption, Value: confirmCreate}},
			}, nil
		},
		Resolve: func(flow.Answers) (flow.Answer, error) {
			return flow.Answer{Value: confirmCreate}, nil
		},
	}
}

func (f *createFlow) sourceUpdate(answers flow.Answers) decide.SourceUpdatePrompt {
	return decide.SourceUpdate(decide.SourceUpdateParams{
		ProjectDir: f.ctx.ProjectDir,
		Target:     f.target,
		Branch:     answers.Value(KeyBranch),
		Source:     answers.Value(KeySource),
	})
}

func (f *createFlow) reusesBranch(answers flow.Answers) bool {
	return f.target(answers.Value(KeyBranch)).State == domain.BranchTargetExisting
}

func (f *createFlow) recap(answers flow.Answers) string {
	source := answers.Value(KeySource)
	branchName := answers.Value(KeyBranch)
	reused := f.reusesBranch(answers)

	envLabel := answers.Value(KeyEnv)
	if envLabel == "" {
		envLabel = domain.EnvSummaryConfigDefault
	}

	branchLabel := branchName
	if reused {
		branchLabel += domain.BranchReusedSuffix
	}

	sourceField := domain.RecapFieldSource
	if reused {
		sourceField = domain.RecapFieldParent
	}

	ffBranch := ""
	if answers.Value(KeySourceUpdate) == updateFastForward {
		ffBranch = f.sourceUpdate(answers).Branch
	}
	sourceLabel := source
	if ffBranch != "" && ffBranch == source {
		sourceLabel += domain.RecapFastForwardSuffix
	}

	var lines []string
	if branchLabel != "" {
		lines = append(lines, domain.RecapFieldBranch+branchLabel)
	}
	lines = append(lines, sourceField+sourceLabel, domain.RecapFieldEnv+envLabel)
	if isolation := answers.Value(KeyIsolation); isolation != "" {
		lines = append(lines, domain.RecapFieldIsolation+rules.IsolationSummary(domain.Isolation(isolation)))
	}
	if ffBranch != "" && ffBranch != source {
		lines = append(lines, fmt.Sprintf(domain.RecapUpdateFastForward, ffBranch))
	}

	if warnings := f.warnings(answers); len(warnings) > 0 {
		lines = append(lines, "")
		lines = append(lines, warnings...)
	}
	return strings.Join(lines, "\n")
}

// warnings are the consequences the user cannot see anywhere else: a source that
// cannot be fast-forwarded, and the parent env strategy falling back to main.
func (f *createFlow) warnings(answers flow.Answers) []string {
	var warnings []string
	if prompt := f.sourceUpdate(answers); prompt.Show && prompt.AbortOnDecline && prompt.Warning != "" {
		warnings = append(warnings, domain.WarningPrefix+prompt.Warning)
	}
	if show, warning := decide.EnvParentFallback(decide.EnvFallbackParams{
		ProjectDir:  f.ctx.ProjectDir,
		Source:      answers.Value(KeySource),
		Config:      f.ctx.Config,
		EnvOverride: answers.Value(KeyEnv),
	}); show {
		warnings = append(warnings, domain.WarningPrefix+warning)
	}
	return warnings
}
