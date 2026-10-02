package extract

import (
	"errors"
	"fmt"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/flow/create"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/worktree"
)

const (
	KeySource = "extract.source"
	KeyFiles  = "extract.files"
	KeyTarget = "extract.target"
	KeyMode   = "extract.mode"
	KeyRecap  = "extract.recap"
)

const (
	// targetCreate is the target answer that creates a worktree: the create steps
	// apply to it alone.
	targetCreate   = "\x00new-worktree"
	modeMove       = "move"
	modeKeep       = "keep"
	confirmExtract = "extract"
)

// The session is flat: create's own steps follow the target, gated on it, so one
// recap covers both the worktree created and what moves into it.
func (f *extractFlow) session() flow.Session {
	steps := []flow.Step{f.sourceStep(), f.filesStep(), f.targetStep()}
	if f.mayCreate() {
		steps = append(steps, f.create.Steps()...)
	}
	steps = append(steps, f.modeStep(), f.recapStep())

	presets := f.create.Presets()
	presets[KeySource] = f.request.Source
	presets[KeyTarget] = f.targetPreset()
	if f.request.KeepSet {
		presets[KeyMode] = modeOf(f.request.Keep)
	}
	return flow.Session{
		ErrLabel: domain.WizardErrLabel,
		Presets:  flow.NewAnswers(presets).WithValues(KeyFiles, f.request.Files),
		Steps:    steps,
	}
}

func (f *extractFlow) targetPreset() string {
	switch {
	case f.request.To == "":
		return ""
	case f.creates:
		return targetCreate
	}
	return f.request.To
}

// mayCreate leaves create's steps out of a session whose --to names a worktree
// already there: the wizard would count them in every breadcrumb.
func (f *extractFlow) mayCreate() bool {
	return f.request.To == "" || f.creates
}

func modeOf(keep bool) string {
	if keep {
		return modeKeep
	}
	return modeMove
}

func (f *extractFlow) createsTarget(answers flow.Answers) bool {
	return answers.Value(KeyTarget) == targetCreate
}

func (f *extractFlow) embed() create.Embedded {
	branch := ""
	if f.creates {
		branch = f.request.To
	}
	return create.Embed(create.EmbedParams{
		Context:      f.ctx,
		Applies:      f.createsTarget,
		Branch:       branch,
		BranchFlag:   domain.FlagTo,
		From:         f.request.From,
		Parent:       f.defaultParent,
		FastForward:  f.request.FastForward,
		Isolation:    f.request.Isolation,
		Target:       f.target,
		SourceUpdate: f.update,
	})
}

// defaultParent is what a new target stacks on: the source's recorded parent,
// whether the source was named or picked. Empty leaves create's base branch.
func (f *extractFlow) defaultParent(answers flow.Answers) string {
	source := answers.Value(KeySource)
	if source == "" {
		return ""
	}
	return worktree.ParentBranch(worktree.ParentBranchParams{StateDir: f.ctx.StateDir, Branch: source})
}

func (f *extractFlow) sourceStep() flow.Step {
	dirty := f.dirty()
	options := make([]flow.Option, 0, len(dirty))
	for _, status := range dirty {
		options = append(options, flow.Option{Label: status.Branch, Value: status.Branch})
	}
	return flow.Step{
		Kind:        flow.StepSelect,
		Key:         KeySource,
		Label:       domain.ExtractSourceLabel,
		Title:       domain.ExtractSourceLabel,
		Description: domain.ExtractSourceDescription,
		Options:     options,
		Resolve: func(flow.Answers) (flow.Answer, error) {
			return flow.Answer{}, domain.ErrExtractSourceRequired
		},
		Arg: true,
	}
}

// filesStep lists a source given up front at once, and a picked one as it is
// picked: git status is proportional to the worktree.
func (f *extractFlow) filesStep() flow.Step {
	step := flow.Step{
		Kind:  flow.StepMultiSelect,
		Key:   KeyFiles,
		Label: domain.ExtractFilesLabel,
		Title: domain.ExtractFilesTitle,
		ValidateSet: func(values []string) error {
			if len(values) == 0 {
				return errors.New(domain.ExtractFilesRequired)
			}
			return nil
		},
		Resolve: func(flow.Answers) (flow.Answer, error) {
			return flow.Answer{}, domain.ErrExtractFilesRequired
		},
		Summarize: func(answer flow.Answer) string { return rules.FileCount(len(answer.Values)) },
		Flag:      domain.FlagFiles,
	}
	if f.request.Source != "" {
		step.Build = f.files
		return step
	}
	step.LoadingMessage = domain.ExtractFilesLoading
	step.Load = f.files
	return step
}

func (f *extractFlow) files(answers flow.Answers) (flow.StepContent, error) {
	source := answers.Value(KeySource)
	files, err := f.pickedChanges(source)
	if err != nil {
		return flow.StepContent{}, err
	}
	if len(files) == 0 {
		return flow.StepContent{Description: fmt.Sprintf(domain.ExtractFilesNoneFmt, source)}, nil
	}
	options := make([]flow.Option, 0, len(files))
	for _, file := range files {
		options = append(options, flow.Option{
			Label: rules.ExtractFileLabel(file),
			Value: file.Path,
			Tag:   rules.ExtractStatusLabel(file.Status),
			Tone:  rules.ExtractStatusTone(file.Status),
		})
	}
	return flow.StepContent{Description: domain.MultiSelectHint, Options: options}, nil
}

func (f *extractFlow) targetStep() flow.Step {
	return flow.Step{
		Kind:        flow.StepSelect,
		Key:         KeyTarget,
		Label:       domain.ExtractTargetLabel,
		Title:       domain.ExtractTargetLabel,
		Description: domain.ExtractTargetDescription,
		Build: func(answers flow.Answers) (flow.StepContent, error) {
			return flow.StepContent{Options: f.targetOptions(answers.Value(KeySource))}, nil
		},
		Resolve: func(flow.Answers) (flow.Answer, error) {
			return flow.Answer{}, domain.ErrExtractTargetRequired
		},
		Summarize: func(answer flow.Answer) string {
			if answer.Value == targetCreate {
				return domain.ExtractTargetCreateSummary
			}
			return answer.Value
		},
		Flag: domain.FlagTo,
	}
}

func (f *extractFlow) targetOptions(source string) []flow.Option {
	options := []flow.Option{{Label: domain.ExtractTargetCreateOption, Value: targetCreate}}
	existing := make([]flow.Option, 0, len(f.statuses))
	for _, status := range f.statuses {
		if status.Branch == source {
			continue
		}
		existing = append(existing, flow.Option{Label: status.Branch, Value: status.Branch})
	}
	if len(existing) == 0 {
		return options
	}
	return append(append(options, flow.Option{Separator: true}), existing...)
}

func (f *extractFlow) modeStep() flow.Step {
	return flow.Step{
		Kind:        flow.StepSelect,
		Key:         KeyMode,
		Label:       domain.ExtractModeLabel,
		Title:       domain.ExtractModeLabel,
		Description: domain.ExtractModeDescription,
		Build: func(answers flow.Answers) (flow.StepContent, error) {
			source := answers.Value(KeySource)
			return flow.StepContent{Options: []flow.Option{
				{Label: fmt.Sprintf(domain.ExtractModeMoveFmt, source), Value: modeMove},
				{Label: fmt.Sprintf(domain.ExtractModeCopyFmt, source), Value: modeKeep},
			}}, nil
		},
		Resolve:   func(flow.Answers) (flow.Answer, error) { return flow.Answer{Value: modeMove}, nil },
		Summarize: func(answer flow.Answer) string { return modeSummary(answer.Value) },
		Flag:      domain.FlagKeep,
	}
}

func modeSummary(mode string) string {
	if mode == modeKeep {
		return domain.ExtractModeCopySummary
	}
	return domain.ExtractModeMoveSummary
}

func (f *extractFlow) recapStep() flow.Step {
	return flow.Step{
		Kind:  flow.StepRecap,
		Key:   KeyRecap,
		Label: domain.ExtractRecapLabel,
		Build: func(answers flow.Answers) (flow.StepContent, error) {
			action := domain.ExtractRecapConfirmOption
			if f.createsTarget(answers) {
				action = domain.ExtractRecapCreateOption
			}
			return flow.StepContent{
				Description: f.recap(answers),
				Options:     []flow.Option{{Label: action, Value: confirmExtract}},
			}, nil
		},
		Resolve: func(flow.Answers) (flow.Answer, error) {
			return flow.Answer{Value: confirmExtract}, nil
		},
	}
}

func (f *extractFlow) recap(answers flow.Answers) string {
	var lines []string
	if source := answers.Value(KeySource); source != "" {
		lines = append(lines, domain.RecapFieldSource+source)
	}
	if files := f.selectedPaths(answers); len(files) > 0 {
		lines = append(lines, domain.RecapFieldFiles+strings.Join(files, ", "))
	}

	var warnings []string
	switch {
	case f.createsTarget(answers):
		plan := f.create.Plan(answers)
		lines = append(lines, createdTargetLines(plan)...)
		warnings = plan.Warnings
	case answers.Value(KeyTarget) != "":
		lines = append(lines, domain.RecapFieldTarget+answers.Value(KeyTarget))
	}

	lines = append(lines, domain.RecapFieldMode+modeSummary(answers.Value(KeyMode)))
	if len(warnings) > 0 {
		lines = append(lines, "")
		lines = append(lines, warnings...)
	}
	return strings.Join(lines, "\n")
}

// selectedPaths names the files a directory in --files stands for, as the
// conclusion will; the answer as given when the changes are not known.
func (f *extractFlow) selectedPaths(answers flow.Answers) []string {
	given := answers.Values(KeyFiles)
	selected, err := rules.SelectExtractFiles(rules.SelectExtractFilesParams{Available: f.changes[answers.Value(KeySource)], Paths: given})
	if err != nil {
		return given
	}
	paths := make([]string, 0, len(selected))
	for _, file := range selected {
		paths = append(paths, file.Path)
	}
	return paths
}

// createdTargetLines says a reused branch is checked out as-is, rather than the
// misleading "new worktree … from …".
func createdTargetLines(plan create.Plan) []string {
	var lines []string
	if plan.Reused {
		lines = append(lines, domain.RecapFieldTarget+plan.Branch+domain.BranchReusedSuffix)
		if parent := plan.From; parent != "" {
			if plan.FastForward == parent {
				parent += domain.RecapFastForwardSuffix
			}
			lines = append(lines, domain.RecapFieldParent+parent)
		}
	} else {
		source := plan.From
		if plan.FastForward != "" && plan.FastForward == plan.From {
			source += domain.RecapFastForwardSuffix
		}
		lines = append(lines, domain.RecapFieldTarget+fmt.Sprintf(domain.ExtractRecapNewTargetFmt, plan.Branch, source))
	}
	if plan.FastForward != "" && plan.FastForward != plan.From {
		lines = append(lines, fmt.Sprintf(domain.RecapUpdateFastForward, plan.FastForward))
	}
	if plan.Isolation != "" {
		lines = append(lines, domain.RecapFieldIsolation+rules.IsolationSummary(plan.Isolation))
	}
	return lines
}
