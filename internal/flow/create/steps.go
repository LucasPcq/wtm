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
	"github.com/LucasPcq/wtm/internal/service/worktree"
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
		Presets:  flow.NewAnswers(f.presets()),
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

func (f *createFlow) presets() map[string]string {
	presets := map[string]string{
		KeySource:    f.request.From,
		KeyEnv:       f.request.EnvFrom,
		KeyIsolation: string(f.request.Isolation),
	}
	// One argument answers the step, as it always did; several pre-fill the list
	// so the wizard can still edit them.
	if len(f.request.Branches) == 1 {
		presets[KeyBranch] = f.request.Branches[0]
	}
	return presets
}

func (f *createFlow) branchStep() flow.Step {
	return flow.Step{
		Kind:        flow.StepTextList,
		Key:         KeyBranch,
		Label:       domain.CreateBranchesLabel,
		Title:       domain.CreateBranchesLabel,
		Description: domain.CreateBranchesStepDescription,
		Build: func(flow.Answers) (flow.StepContent, error) {
			return flow.StepContent{Entries: f.request.Branches}, nil
		},
		ValidateEntry: f.validateEntry,
		EntryBadge:    f.entryBadge,
		ValidateSet: func(values []string) error {
			if len(values) == 0 {
				return errors.New(domain.CreateBranchRequired)
			}
			return nil
		},
		Resolve: func(flow.Answers) (flow.Answer, error) {
			if len(f.request.Branches) == 0 {
				return flow.Answer{}, errors.New(domain.CreateBranchRequiredUnattended)
			}
			return flow.Answer{Values: f.request.Branches}, nil
		},
		Summarize: flow.SummarizeSet,
		Arg:       true,
	}
}

func (f *createFlow) validateEntry(check flow.EntryCheck) error {
	if err := rules.BranchEntryProblem(rules.BranchEntryParams{
		Entry:        check.Entry,
		Entries:      check.Entries,
		DerivedNames: f.derivedNames,
	}); err != nil {
		return err
	}
	if target := f.target(check.Entry); target.State == domain.BranchTargetCheckedOut && !f.request.IfNotExists {
		return fmt.Errorf("%w: "+domain.BranchCheckedOutElsewhereFmt,
			domain.ErrWorktreeExists, check.Entry, target.WorktreePath, check.Entry)
	}
	if !f.derivedNames {
		return nil
	}
	return worktree.CheckNameFree(worktree.NameCheckParams{ProjectDir: f.ctx.ProjectDir, StateDir: f.ctx.StateDir, Branch: check.Entry})
}

func (f *createFlow) entryBadge(entry string) flow.Badge {
	switch f.target(entry).State {
	case domain.BranchTargetExisting:
		return flow.Badge{Text: domain.BranchEntryExisting, Tone: domain.ToneWarning}
	case domain.BranchTargetCheckedOut:
		return flow.Badge{Text: domain.BranchEntryWorktreeExists, Tone: domain.ToneNeutral}
	}
	return flow.Badge{Text: domain.BranchEntryNew, Tone: domain.ToneSuccess}
}

func (f *createFlow) branches(answers flow.Answers) []string { return answers.Values(KeyBranch) }

func (f *createFlow) many(answers flow.Answers) bool { return len(f.branches(answers)) > 1 }

func (f *createFlow) existingBranches(answers flow.Answers) []string {
	var found []string
	for _, name := range f.branches(answers) {
		if f.target(name).State == domain.BranchTargetExisting {
			found = append(found, name)
		}
	}
	return found
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
			if f.many(answers) {
				description = domain.CreateSourceStepDescriptionMany
			}
			switch {
			case f.reusesBranch(answers) && len(f.branches(answers)) > 1:
				description = domain.RecapParentRecordedForExisting
			case f.reusesBranch(answers):
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
	for _, name := range f.branches(answers) {
		if rules.ParentMustBeExplicit(f.target(name).State) {
			return flow.Answer{}, fmt.Errorf(domain.ParentRequiredFmt, name, domain.FlagFrom)
		}
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
		Build: func(answers flow.Answers) (flow.StepContent, error) {
			return flow.StepContent{Description: decide.Pick(decide.PickParams{
				Many:    f.many(answers),
				One:     domain.CreateEnvStepDescription,
				Several: domain.CreateEnvStepDescriptionMany,
			})}, nil
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
		Options: isolationOptions(isolationOptionsParams{First: fallback}),
		Build: func(answers flow.Answers) (flow.StepContent, error) {
			many := f.many(answers)
			return flow.StepContent{
				Description: decide.Pick(decide.PickParams{Many: many, One: domain.IsolationStepDescription, Several: domain.IsolationStepDescriptionMany}),
				Options:     isolationOptions(isolationOptionsParams{First: fallback, Many: many}),
			}, nil
		},
		Resolve: func(flow.Answers) (flow.Answer, error) {
			return flow.Answer{Value: string(fallback)}, nil
		},
		Summarize: func(answer flow.Answer) string { return rules.IsolationSummary(domain.Isolation(answer.Value)) },
		Flag:      domain.FlagIsolation,
	}
}

type isolationOptionsParams struct {
	First domain.Isolation
	Many  bool
}

func isolationOptions(params isolationOptionsParams) []flow.Option {
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
				Options:     []flow.Option{{Label: confirmLabel(len(f.branches(answers))), Value: confirmCreate}},
			}, nil
		},
		Resolve: func(flow.Answers) (flow.Answer, error) {
			return flow.Answer{Value: confirmCreate}, nil
		},
	}
}

// sourceUpdate moves the branch itself only when it is the one being created: a
// list shares its source, and that is what gets fast-forwarded.
func (f *createFlow) sourceUpdate(answers flow.Answers) decide.SourceUpdatePrompt {
	single := ""
	if names := f.branches(answers); len(names) == 1 {
		single = names[0]
	}
	return decide.SourceUpdate(decide.SourceUpdateParams{
		ProjectDir: f.ctx.ProjectDir,
		Target:     f.target,
		Branch:     single,
		Source:     answers.Value(KeySource),
		Many:       f.many(answers),
	})
}

func (f *createFlow) reusesBranch(answers flow.Answers) bool {
	return len(f.existingBranches(answers)) > 0
}

func (f *createFlow) allReused(answers flow.Answers) bool {
	names := f.branches(answers)
	return len(names) > 0 && len(f.existingBranches(answers)) == len(names)
}

func (f *createFlow) branchLine(answers flow.Answers) string {
	names := f.branches(answers)
	switch {
	case len(names) == 1:
		label := names[0]
		if f.reusesBranch(answers) {
			label += domain.BranchReusedSuffix
		}
		return domain.RecapFieldBranch + label
	case len(names) > 1:
		labels := make([]string, 0, len(names))
		for _, name := range names {
			if f.target(name).State == domain.BranchTargetExisting {
				name += domain.BranchListExistingSuffix
			}
			labels = append(labels, name)
		}
		return domain.RecapFieldBranches + strings.Join(labels, ", ")
	}
	return ""
}

func (f *createFlow) recap(answers flow.Answers) string {
	source := answers.Value(KeySource)

	envLabel := answers.Value(KeyEnv)
	if envLabel == "" {
		envLabel = domain.EnvSummaryConfigDefault
	}

	sourceField := domain.RecapFieldSource
	if f.allReused(answers) {
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
	if line := f.branchLine(answers); line != "" {
		lines = append(lines, line)
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

func confirmLabel(count int) string {
	if count > 1 {
		return fmt.Sprintf(domain.CreateRecapConfirmManyFmt, count)
	}
	return domain.CreateRecapConfirmOption
}
