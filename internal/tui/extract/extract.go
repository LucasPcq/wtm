// Package extract renders the interactive selection flow for `wtm extract`:
// a single wizard whose steps are the file multi-select and the target worktree
// picker, so the breadcrumb, step summaries, and back navigation all work.
package extract

import (
	"errors"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/tui/components"
	newpicker "github.com/LucasPcq/wtm/internal/tui/newwt"
)

// newWorktreeValue is the sentinel SelectList value for the "create new" entry.
const newWorktreeValue = "\x00new-worktree"

// TargetChoice is the outcome of the target selection.
type TargetChoice struct {
	CreateNew bool
	Branch    string
}

// RunParams holds inputs for the interactive extraction wizard. NeedSource,
// NeedFiles, NeedTarget and NeedMode toggle the corresponding steps, letting the
// command skip a step already resolved from a flag or the positional argument.
type RunParams struct {
	// Files are the source worktree's changes when the source is fixed (an argument
	// was given, NeedSource false). When NeedSource, the changes are loaded lazily
	// per picked worktree via LoadFiles instead.
	Files     []domain.ExtractFile
	Worktrees []domain.WorktreeStatus
	// SourceBranch fixes the source when NeedSource is false; empty otherwise (the
	// source is then chosen on the first step).
	SourceBranch string
	// Sources are the candidate source worktrees (those with changes) for the source
	// picker. Only used when NeedSource.
	Sources []domain.WorktreeStatus
	// LoadFiles returns the changes of a source worktree, injected by the command so
	// the TUI stays free of git/service imports. Only used when NeedSource.
	LoadFiles  func(branch string) []domain.ExtractFile
	NeedSource bool
	NeedFiles  bool
	NeedTarget bool
	NeedMode   bool
	// FixedFiles, FixedTarget and FixedKeep carry the values resolved from the
	// --files/--to/--keep flags, so the combined recap names every part of the plan
	// even for the steps a flag skipped. Each is used only when its step is absent.
	FixedFiles  []string
	FixedTarget string
	FixedKeep   bool
	// Create configures the embedded "create new worktree" sub-flow, inserted after
	// the target step and auto-skipped unless the target is "create new". Only used
	// when NeedTarget. Its Source is "" (source picked), IncludeBranch true,
	// IncludeEnv false (the new worktree uses the config-default env strategy).
	Create newpicker.WizardParams
}

// RunResult holds the wizard answers. Only the fields for the requested steps
// are populated.
type RunResult struct {
	// SourceBranch is the picked source worktree branch when NeedSource; empty
	// otherwise (the command already knows the fixed source).
	SourceBranch string
	Files        []string
	Target       TargetChoice
	Keep         bool
	// Create holds the new-worktree answers (branch/source/fast-forward) when
	// Target.CreateNew; zero otherwise.
	Create newpicker.WizardResult
}

const (
	modeMove = "move"
	modeKeep = "keep"
)

// Step name and action value for the final recap.
const (
	stepConfirm    = "Confirm"
	extractConfirm = "extract"
)

// Step names, used for construction and name-based value extraction.
const (
	stepSource = "Source worktree"
	stepFiles  = "Files"
	stepTarget = "Target worktree"
	stepMode   = "Mode"
)

// filesRequestMsg asks the message handler to load the changes of the picked
// source worktree; filesDoneMsg carries them back. The changes are loaded lazily
// (git status per worktree is proportional to the selection) behind the shared
// loading spinner, mirroring clean's async safety check.
type (
	filesRequestMsg struct{ branch string }
	filesDoneMsg    struct {
		branch string
		files  []domain.ExtractFile
	}
)

// Run shows the extraction wizard and returns the selected files, target, and —
// when the target is "create new" — the new-worktree answers. The create sub-flow
// is embedded so a single combined recap covers both create and extract. Returns
// domain.ErrUserAborted when the user cancels.
func Run(params RunParams) (RunResult, error) {
	var steps []components.Step

	if params.NeedSource {
		steps = append(steps, components.Step{
			Name:    stepSource,
			Model:   newSourceSelect(params.Sources),
			Summary: components.SelectSummary,
		})
	}

	// The Files step index in the final steps slice, so the message handler can swap
	// the lazily-loaded changes in. -1 when there is no Files step.
	filesIdx := -1
	if params.NeedFiles {
		filesIdx = len(steps)
		steps = append(steps, filesStep(params))
	}
	if params.NeedTarget {
		steps = append(steps, targetStep(params))
	}

	// The create sub-flow, inserted after the target step and auto-skipped unless
	// the target is "create new" — so creating the worktree and extracting share
	// one combined recap instead of two.
	var createFlow newpicker.CreateFlow
	if params.NeedTarget {
		createFlow = newpicker.CreateSteps(params.Create, isNewTarget)
		steps = append(steps, createFlow.Steps...)
	}

	if params.NeedMode {
		steps = append(steps, modeStep(params))
	}

	// Combined recap: source + files + target (existing branch, or "new worktree
	// <branch> from <source>") + mode + ⚠ create warnings, then "No, cancel". This
	// wizard only runs fully interactively (--yes / no-TTY / JSON never reach it), so
	// the recap is always the last step.
	steps = append(steps, components.RecapStep(components.RecapStepParams{
		Name:  stepConfirm,
		Build: func(prev []components.Step) components.RecapContent { return buildCombinedRecap(prev, params) },
	}))

	final, err := components.RunWizard(components.RunWizardParams{
		Steps:       steps,
		ErrLabel:    "extract wizard",
		OnMsg:       combineMsgHandlers(filesMsgHandler(filesIdx, params.LoadFiles), createFlow.OnMsg),
		InitCmd:     createFlow.InitCmd,
		Loading:     createFlow.InitCmd != nil,
		LoadingText: createFlow.LoadingText,
	})
	if err != nil {
		return RunResult{}, err
	}
	steps = final.Steps()
	if v, _ := stepSelectValue(steps, stepConfirm); v == domain.WizardCancelValue {
		return RunResult{}, domain.ErrUserAborted
	}

	var result RunResult
	if v, ok := stepSelectValue(steps, stepSource); ok {
		result.SourceBranch = v
	}
	if ms, ok := stepModelByName(steps, stepFiles).(components.MultiSelectModel); ok {
		result.Files = ms.Values()
	}
	if v, ok := stepSelectValue(steps, stepTarget); ok {
		result.Target = parseTarget(v)
		if result.Target.CreateNew {
			result.Create = newpicker.ReadCreateResult(steps, params.Create)
		}
	}
	if v, ok := stepSelectValue(steps, stepMode); ok {
		result.Keep = v == modeKeep
	}
	return result, nil
}

// filesStep builds the file multi-select. When the source is picked (NeedSource),
// its model starts empty and its changes are loaded lazily on entry via OnEnter +
// the message handler; when the source is fixed, the changes are known up front.
func filesStep(params RunParams) components.Step {
	if !params.NeedSource {
		return components.Step{
			Name:    stepFiles,
			Model:   newFileSelect(params.Files, nil),
			Summary: fileSummary,
		}
	}
	return components.Step{
		Name:    stepFiles,
		Model:   newFileSelect(nil, nil),
		Summary: fileSummary,
		OnEnter: func(prev []components.Step) tea.Cmd {
			branch, _ := stepSelectValue(prev, stepSource)
			return func() tea.Msg { return filesRequestMsg{branch: branch} }
		},
	}
}

// targetStep builds the target picker. When the source is picked it rebuilds on
// entry to exclude the chosen source; when fixed, the source is known up front.
func targetStep(params RunParams) components.Step {
	step := components.Step{
		Name:    stepTarget,
		Model:   newTargetSelect(params.Worktrees, params.SourceBranch),
		Summary: targetSummary,
	}
	if params.NeedSource {
		step.Build = func(prev []components.Step) any {
			return newTargetSelect(params.Worktrees, sourceBranchFromPrev(prev, params.SourceBranch))
		}
	}
	return step
}

// modeStep builds the move/copy select. When the source is picked its label is
// rebuilt on entry to name the chosen source branch.
func modeStep(params RunParams) components.Step {
	step := components.Step{
		Name:    stepMode,
		Model:   newModeSelect(newModeSelectParams{SourceBranch: params.SourceBranch}),
		Summary: modeSummary,
	}
	if params.NeedSource {
		step.Build = func(prev []components.Step) any {
			return newModeSelect(newModeSelectParams{SourceBranch: sourceBranchFromPrev(prev, params.SourceBranch)})
		}
	}
	return step
}

// filesMsgHandler lazily loads the picked source worktree's changes into the Files
// step behind the shared spinner, mirroring clean's async safety check. It is a
// no-op (returns unhandled) when there is no Files step or no loader.
func filesMsgHandler(filesIdx int, load func(branch string) []domain.ExtractFile) components.WizardMsgHandler {
	if filesIdx < 0 || load == nil {
		return nil
	}
	return func(w *components.WizardModel, msg tea.Msg) (tea.Cmd, bool) {
		switch m := msg.(type) {
		case filesRequestMsg:
			// Clear any changes left from a previously-picked source so the spinner
			// isn't shown over a stale list while the new load runs.
			w.UpdateStepModel(filesIdx, func(any) any { return newFileSelect(nil, nil) })
			loadCmd := w.StartLoading("Loading changes…")
			return tea.Batch(loadCmd, loadFilesCmd(m.branch, load)), true
		case filesDoneMsg:
			w.UpdateStepModel(filesIdx, func(any) any { return filesModel(m.branch, m.files) })
			w.SetLoading(false)
			return nil, true
		}
		return nil, false
	}
}

// combineMsgHandlers runs first, then next, returning on the first that consumes
// the message. Either may be nil.
func combineMsgHandlers(first, next components.WizardMsgHandler) components.WizardMsgHandler {
	if first == nil {
		return next
	}
	if next == nil {
		return first
	}
	return func(w *components.WizardModel, msg tea.Msg) (tea.Cmd, bool) {
		if cmd, handled := first(w, msg); handled {
			return cmd, true
		}
		return next(w, msg)
	}
}

// loadFilesCmd runs the (git) change listing off the UI goroutine so the spinner
// animates while it runs.
func loadFilesCmd(branch string, load func(branch string) []domain.ExtractFile) tea.Cmd {
	return func() tea.Msg { return filesDoneMsg{branch: branch, files: load(branch)} }
}

// filesModel builds the loaded file multi-select, or an empty placeholder naming
// the source when it has no changes (Validate then blocks Enter; Esc goes back).
func filesModel(branch string, files []domain.ExtractFile) components.MultiSelectModel {
	if len(files) == 0 {
		return components.NewMultiSelect(components.NewMultiSelectParams{
			Title:       "Select files to extract",
			Description: "No changes to extract in " + branch + " — press esc to pick another worktree.",
			Validate:    func([]string) error { return fmt.Errorf("no changes to extract in %s", branch) },
		})
	}
	return newFileSelect(files, nil)
}

// sourceBranchFromPrev returns the picked source branch, or the fixed fallback
// when there is no source step.
func sourceBranchFromPrev(prev []components.Step, fixed string) string {
	if v, ok := stepSelectValue(prev, stepSource); ok {
		return v
	}
	return fixed
}

// newSourceSelect lists the candidate source worktrees (those with changes).
func newSourceSelect(sources []domain.WorktreeStatus) components.SelectListModel {
	items := make([]components.SelectItem, 0, len(sources))
	for _, s := range sources {
		items = append(items, components.SelectItem{Label: s.Branch, Value: s.Branch})
	}
	return components.NewSelectList(components.NewSelectListParams{
		Title:       "Source worktree",
		Description: "Which worktree to extract changes from",
		Items:       items,
	})
}

// isNewTarget reports whether the chosen target is the "create new worktree" entry
// — the gate for the embedded create steps.
func isNewTarget(steps []components.Step) bool {
	v, _ := stepSelectValue(steps, stepTarget)
	return v == newWorktreeValue
}

// buildCombinedRecap recaps whichever of the file/target/mode steps are present,
// expanding a "create new" target into its branch/source and folding in the create
// warnings.
func buildCombinedRecap(prev []components.Step, params RunParams) components.RecapContent {
	var lines []string
	if source := sourceBranchFromPrev(prev, params.SourceBranch); source != "" {
		lines = append(lines, domain.RecapFieldSource+source)
	}
	if ms, ok := stepModelByName(prev, stepFiles).(components.MultiSelectModel); ok {
		lines = append(lines, domain.RecapFieldFiles+strings.Join(ms.Values(), ", "))
	} else if len(params.FixedFiles) > 0 {
		lines = append(lines, domain.RecapFieldFiles+strings.Join(params.FixedFiles, ", "))
	}

	action := "extract"
	if sl, ok := stepModelByName(prev, stepTarget).(components.SelectListModel); ok {
		if isNewTarget(prev) {
			cr := newpicker.ReadCreateResult(prev, params.Create)
			// A reused branch is checked out as-is, not created from a start-point —
			// the recap says so instead of the misleading "new worktree ... from ...".
			if cr.Reused {
				lines = append(lines, domain.RecapFieldTarget+cr.BranchName+domain.BranchReusedSuffix)
				if parent := cr.FromBranch; parent != "" {
					if cr.FastForwardBranch == parent {
						parent += " (fast-forward to origin)"
					}
					lines = append(lines, domain.RecapFieldParent+parent)
				}
			} else {
				source := cr.FromBranch
				if cr.FastForwardBranch == cr.FromBranch && cr.FastForwardBranch != "" {
					source += " (fast-forward to origin)"
				}
				lines = append(lines, domain.RecapFieldTarget+"new worktree "+cr.BranchName+" from "+source)
			}
			if cr.FastForwardBranch != "" && cr.FastForwardBranch != cr.FromBranch {
				lines = append(lines, fmt.Sprintf(domain.RecapUpdateFastForward, cr.FastForwardBranch))
			}
			if line, shown := newpicker.IsolationRecapLine(prev, params.Create); shown {
				lines = append(lines, line)
			}
			action = "create & extract"
		} else {
			lines = append(lines, domain.RecapFieldTarget+targetSummary(sl))
		}
	} else if params.FixedTarget != "" {
		lines = append(lines, domain.RecapFieldTarget+params.FixedTarget)
	}

	if sl, ok := stepModelByName(prev, stepMode).(components.SelectListModel); ok {
		lines = append(lines, domain.RecapFieldMode+modeSummary(sl))
	} else if !params.NeedMode {
		lines = append(lines, domain.RecapFieldMode+fixedModeLabel(params.FixedKeep))
	}

	if isNewTarget(prev) {
		if warnings := newpicker.CreateWarnings(prev, params.Create); len(warnings) > 0 {
			lines = append(lines, "")
			lines = append(lines, warnings...)
		}
	}

	return components.RecapContent{
		Description: strings.Join(lines, "\n"),
		Actions:     []components.SelectItem{{Label: "Yes, " + action, Value: extractConfirm}},
	}
}

// stepSelectValue reads the chosen value of the named SelectList step.
func stepSelectValue(steps []components.Step, name string) (string, bool) {
	if sl, ok := stepModelByName(steps, name).(components.SelectListModel); ok {
		return sl.Value(), true
	}
	return "", false
}

// stepModelByName returns the model of the named step, or nil when absent.
func stepModelByName(steps []components.Step, name string) any {
	for _, s := range steps {
		if s.Name == name {
			return s.Model
		}
	}
	return nil
}

func newFileSelect(files []domain.ExtractFile, preselected []string) components.MultiSelectModel {
	chosen := make(map[string]struct{}, len(preselected))
	for _, p := range preselected {
		chosen[p] = struct{}{}
	}

	items := make([]components.MultiSelectItem, 0, len(files))
	for _, f := range files {
		_, isChosen := chosen[f.Path]
		items = append(items, components.MultiSelectItem{
			Label:    fileLabel(f),
			Value:    f.Path,
			Tag:      rules.ExtractStatusLabel(f.Status),
			Variant:  tagVariant(f.Status),
			Selected: isChosen,
		})
	}

	return components.NewMultiSelect(components.NewMultiSelectParams{
		Title:       "Select files to extract",
		Description: domain.MultiSelectHint,
		Items:       items,
		Validate: func(vals []string) error {
			if len(vals) == 0 {
				return fmt.Errorf("select at least one file")
			}
			return nil
		},
	})
}

func newTargetSelect(worktrees []domain.WorktreeStatus, sourceBranch string) components.SelectListModel {
	items := []components.SelectItem{
		{Label: "+ Create a new worktree…", Value: newWorktreeValue},
	}

	existing := make([]components.SelectItem, 0, len(worktrees))
	for _, w := range worktrees {
		if w.Branch == sourceBranch {
			continue
		}
		existing = append(existing, components.SelectItem{Label: w.Branch, Value: w.Branch})
	}
	if len(existing) > 0 {
		items = append(items, components.SelectItem{Separator: true})
		items = append(items, existing...)
	}

	return components.NewSelectList(components.NewSelectListParams{
		Title:       "Target worktree",
		Description: "Where to move the selected files",
		Items:       items,
	})
}

// newModeSelectParams holds inputs for newModeSelect.
type newModeSelectParams struct {
	SourceBranch string
}

func newModeSelect(params newModeSelectParams) components.SelectListModel {
	return components.NewSelectList(components.NewSelectListParams{
		Title:       "Mode",
		Description: "Move removes the files from the source; copy keeps them.",
		Items: []components.SelectItem{
			{Label: "Move — remove the files from " + params.SourceBranch, Value: modeMove},
			{Label: "Copy — keep the files in " + params.SourceBranch, Value: modeKeep},
		},
	})
}

// ConfirmResolve asks whether to apply conflict markers into the target, making
// the consequences explicit. Returns false on No/Esc.
func ConfirmResolve(files []string, targetBranch string) (bool, error) {
	desc := fmt.Sprintf(
		"%s already present in %q.\n\n"+
			"Applying writes conflict markers there to resolve.\n"+
			"Nothing is removed from the source.\n"+
			"Resolve in %q then discard there, or discard in %q to undo.",
		joinFiles(files), targetBranch, targetBranch, targetBranch)

	cm := components.NewConfirm(components.NewConfirmParams{
		Title:       "Apply conflict markers in " + targetBranch + "?",
		Description: desc,
	})
	confirmed, err := components.RunStandaloneConfirm(cm)
	if errors.Is(err, components.ErrAborted) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return confirmed, nil
}

func joinFiles(files []string) string {
	if len(files) == 1 {
		return files[0] + " was"
	}
	return strings.Join(files, ", ") + " were"
}

func fileSummary(model any) string {
	ms, ok := model.(components.MultiSelectModel)
	if !ok {
		return ""
	}
	n := len(ms.Values())
	if n == 1 {
		return "1 file"
	}
	return fmt.Sprintf("%d files", n)
}

func targetSummary(model any) string {
	sl, ok := model.(components.SelectListModel)
	if !ok {
		return ""
	}
	if sl.Value() == newWorktreeValue {
		return "new worktree"
	}
	return sl.Value()
}

func modeSummary(model any) string {
	sl, ok := model.(components.SelectListModel)
	if !ok {
		return ""
	}
	if sl.Value() == modeKeep {
		return "copy"
	}
	return "move"
}

// fixedModeLabel labels the move/copy mode fixed by --keep for the recap, when the
// mode step was skipped.
func fixedModeLabel(keep bool) string {
	if keep {
		return "copy"
	}
	return "move"
}

func parseTarget(value string) TargetChoice {
	if value == newWorktreeValue {
		return TargetChoice{CreateNew: true}
	}
	return TargetChoice{Branch: value}
}

// tagVariant maps a file's extraction status to its tag color: yellow for a
// modification, green for a new file, red for a deletion.
// fileLabel is what the picker shows for a candidate. A rename names both of its
// paths, since both take part in the extraction.
func fileLabel(f domain.ExtractFile) string {
	if f.OrigPath == "" {
		return f.Path
	}
	return f.OrigPath + " → " + f.Path
}

func tagVariant(status domain.ExtractFileStatus) components.TagVariant {
	switch status {
	case domain.ExtractStatusUntracked:
		return components.TagSuccess
	case domain.ExtractStatusDeleted:
		return components.TagDanger
	case domain.ExtractStatusRenamed:
		return components.TagNeutral
	default:
		return components.TagWarning
	}
}
