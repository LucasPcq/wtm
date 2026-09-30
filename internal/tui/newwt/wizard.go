// Package newwt builds interactive wizards for the wtm new command.
package newwt

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/tui/branchrefresh"
	"github.com/LucasPcq/wtm/internal/tui/components"
)

// Step names, shared between construction and value extraction so prior-step
// answers can be located by name rather than by a fragile positional offset.
const (
	stepBranchName = "Branch name"
	stepSourceName = "Source branch"
	stepEnvName    = "Env strategy"
	stepSourceUpd  = "Source update"
	stepConfirm    = "Confirm & create"
)

// Source-update choices and the recap action value.
const (
	sourceFastForward = "ff"
	sourceKeep        = "keep"
	createConfirm     = "create"
)

// SourceUpdateParams identifies the branches a reconcile decision arbitrates
// between: Branch is the worktree's own branch (which may already exist locally,
// in which case it — not the source — is what needs updating), Source is the
// start-point / parent.
type SourceUpdateParams struct {
	Branch string
	Source string
}

// SourceUpdatePrompt decides the conditional "reconcile source" step. In the
// wizard, only a fast-forward offer (Show && !AbortOnDecline) is an actionable
// choice; a diverged source (Show && AbortOnDecline) is surfaced as a ⚠ line in
// the recap instead of a gate. The standalone --from path still renders Params as
// a single confirm, so those fields are preserved.
type SourceUpdatePrompt struct {
	// Branch names the branch an accepted fast-forward applies to — the source for
	// a new branch, the worktree's own branch when it already exists locally.
	Branch string
	// Show controls whether the confirmation appears at all (standalone path) or a
	// fast-forward choice appears (wizard path).
	Show bool
	// Params is the prompt to render when Show is true.
	Params components.NewConfirmParams
	// AbortOnDecline treats a "No" as cancelling creation (a diverged source);
	// when false a "No" silently proceeds (declining a fast-forward offer).
	AbortOnDecline bool
	// SkipReason is shown in the wizard summaries when the source-update step is
	// skipped (no fast-forward offer): e.g. "source already up to date".
	SkipReason string
}

// WizardParams holds inputs for the interactive new worktree wizard.
type WizardParams struct {
	ProjectDir     string
	Branches       []domain.BranchCandidate
	DefaultBranch  string
	ConfigStrategy domain.EnvStrategy
	IncludeBranch  bool
	// BranchName is the branch name fixed by a positional arg (when IncludeBranch is
	// false); shown in the recap so it stays complete even without a branch step.
	BranchName string
	// Source, when non-empty (--from), fixes the source branch and skips the source
	// picker. The source-update fast-forward is then computed up front.
	Source string
	// IncludeEnv shows the env-strategy step; false when --env-from fixes it, in
	// which case EnvOverride carries the chosen strategy.
	IncludeEnv  bool
	EnvOverride string
	// IsolationApplies says run.toml declares something a worktree isolates; the
	// command layer reads it, so the TUI stays free of config loading. The step
	// is posed when it applies and --isolation did not answer it.
	IsolationApplies  bool
	IsolationOverride domain.Isolation
	// IsolationDefault is the project's answer, offered first and used wherever
	// nobody was asked.
	IsolationDefault domain.Isolation
	// SourceUpdate, when set, decides the source-update step: given the worktree's
	// branch and its source it offers a fast-forward (behind-only) or reports a
	// diverged branch as a ⚠ recap line. Injected by the command layer so the TUI
	// stays free of git logic.
	SourceUpdate func(SourceUpdateParams) SourceUpdatePrompt
	// Target, when set, reports whether the worktree's branch already exists
	// locally, so the source step, the recap and the warnings speak of a reused
	// branch and a sync parent rather than a start-point. Injected by the command
	// layer so the TUI stays free of git logic.
	Target func(branch string) domain.BranchTarget
	// EnvFallback, when set, decides whether the "parent" env strategy will fall
	// back to copying .env from main — surfaced as a ⚠ recap line.
	EnvFallback func(source, envOverride string) (show bool, params components.NewConfirmParams)
}

// WizardResult holds the answers from the interactive wizard.
type WizardResult struct {
	BranchName      string
	FromBranch      string
	EnvFromOverride string
	// FastForwardBranch names the branch whose fast-forward offer the user
	// accepted — the source for a new branch, the worktree's own branch when it
	// already exists locally. Empty when nothing is to be fast-forwarded.
	FastForwardBranch string
	// Reused reports whether BranchName is an existing local branch that will be
	// checked out as-is rather than created — the host recap uses it to say so
	// instead of a misleading "from <source>".
	Reused    bool
	Isolation domain.Isolation
}

// CreateFlow is the create sub-flow (branch/source/source-update steps) ready to
// embed in a host wizard — e.g. `wtm extract` inserts it before its own recap so
// creating the target worktree and extracting share one combined recap. It also
// carries the source picker's background branch-refresh wiring, which the host
// plugs into its wizard.
type CreateFlow struct {
	Steps       []components.Step
	InitCmd     tea.Cmd
	OnMsg       components.WizardMsgHandler
	LoadingText string
}

// CreateSteps builds the create sub-flow steps WITHOUT the final recap, so a host
// wizard can compose them with its own steps and show one combined recap. When
// enabled is non-nil each step auto-skips while enabled(steps) is false (e.g. the
// extract target is an existing worktree, not "create new"). A nil enabled keeps
// every step active (the standalone `wtm create`).
func CreateSteps(params WizardParams, enabled func(steps []components.Step) bool) CreateFlow {
	holder := &params.Branches
	var steps []components.Step

	if params.IncludeBranch {
		steps = append(steps, gateStep(branchStep(), enabled))
	}
	if params.Source == "" {
		steps = append(steps, gateStep(sourceStep(sourceStepParams{
			Holder:        holder,
			DefaultBranch: params.DefaultBranch,
			Wizard:        params,
		}), enabled))
	}
	if params.IncludeEnv {
		steps = append(steps, gateStep(envStep(params.ConfigStrategy), enabled))
	}
	if includesIsolation(params) {
		steps = append(steps, gateStep(components.IsolationStep(params.IsolationDefault), enabled))
	}
	// The source-update fast-forward: a ChoiceStep whenever a step precedes it (it
	// cannot be a first step), else — source fixed by --from and branch fixed by
	// the positional arg — a concrete step added only when a fast-forward is
	// offered. A diverged branch is not a gate here; it becomes a ⚠ recap line.
	if params.SourceUpdate != nil {
		decide := func(prev []components.Step) SourceUpdatePrompt {
			return params.SourceUpdate(SourceUpdateParams{
				Branch: resolveBranchName(prev, params),
				Source: resolveSource(prev, params),
			})
		}
		switch {
		case params.Source == "" || params.IncludeBranch:
			steps = append(steps, gateStep(sourceUpdateStep(decide), enabled))
		default:
			if p := decide(nil); p.Show && !p.AbortOnDecline {
				steps = append(steps, gateStep(sourceUpdateConcreteStep(p), enabled))
			}
		}
	}

	flow := CreateFlow{Steps: steps}
	if params.Source == "" {
		flow.InitCmd = branchrefresh.Cmd(params.ProjectDir)
		flow.OnMsg = branchrefresh.Handler(params.ProjectDir, holder)
		flow.LoadingText = domain.LoadingBranchesText
	}
	return flow
}

// gateStep makes a step auto-skip while enabled(steps) is false, composing with any
// AutoSkip the step already has (so a ChoiceStep still skips on its own logic when
// enabled). A nil enabled leaves the step untouched.
func gateStep(step components.Step, enabled func(steps []components.Step) bool) components.Step {
	if enabled == nil {
		return step
	}
	inner := step.AutoSkip
	step.AutoSkip = func(w components.WizardModel) bool {
		if !enabled(w.Steps()) {
			return true
		}
		return inner != nil && inner(w)
	}
	return step
}

// ReadCreateResult reads the create answers from a (host or standalone) wizard's
// steps by name. It does not interpret a cancel — the host's recap owns that.
func ReadCreateResult(steps []components.Step, params WizardParams) WizardResult {
	result := WizardResult{
		FromBranch:        resolveSource(steps, params),
		EnvFromOverride:   resolveEnv(steps, params),
		FastForwardBranch: fastForwardBranch(steps, params),
		Reused:            reusesBranch(steps, params),
		Isolation:         resolveIsolation(steps, params),
	}
	if child, ok := stepModelByName(steps, stepBranchName).(components.TextInputModel); ok {
		result.BranchName = child.Value()
	}
	return result
}

func branchStep() components.Step {
	return components.Step{
		Name: stepBranchName,
		Model: components.NewTextInput(components.NewTextInputParams{
			Title:       stepBranchName,
			Description: "Name for the new worktree branch",
			Validate: func(s string) error {
				if strings.TrimSpace(s) == "" {
					return fmt.Errorf("branch name is required")
				}
				return nil
			},
		}),
		Summary: components.TextSummary,
	}
}

// sourceStepParams holds inputs for sourceStep. Wizard carries the branch-name
// answer and the Target decider, so the step can retitle itself once the branch
// is known to already exist.
type sourceStepParams struct {
	Holder        *[]domain.BranchCandidate
	DefaultBranch string
	Wizard        WizardParams
}

func sourceStep(params sourceStepParams) components.Step {
	sourceList := func(prev []components.Step) any {
		description := "Branch to base the new worktree on"
		if reusesBranch(prev, params.Wizard) {
			description = domain.RecapParentRecordedForSync
		}
		return components.NewSelectList(components.NewSelectListParams{
			Title:       stepSourceName,
			Description: description,
			Items:       buildBranchItems(*params.Holder, params.DefaultBranch),
		})
	}
	return components.Step{
		Name:       stepSourceName,
		Model:      sourceList(nil),
		Build:      sourceList,
		CanRefresh: true,
		Summary:    components.SelectSummary,
	}
}

func envStep(strategy domain.EnvStrategy) components.Step {
	return components.Step{
		Name: stepEnvName,
		Model: components.NewSelectList(components.NewSelectListParams{
			Title:       stepEnvName,
			Description: "How to provision .env files in the new worktree",
			Items:       buildEnvItems(strategy),
		}),
		Summary: envStrategySummary,
	}
}

// sourceUpdateItems are the fast-forward-vs-keep options, shared by the ChoiceStep
// (picked source) and concrete (fixed source) variants. The branch is named in the
// label because the offer targets the source for a new branch and the worktree's
// own branch when it already exists.
func sourceUpdateItems(branch string) []components.SelectItem {
	return []components.SelectItem{
		{Label: fmt.Sprintf(domain.SourceFastForwardOptionFmt, branch), Value: sourceFastForward},
		{Separator: true},
		{Label: "Keep it as-is", Value: sourceKeep},
	}
}

// sourceUpdateConcreteStep builds an always-shown fast-forward select for the
// --from path, where the source (and thus its divergence) is known up front. It
// avoids ChoiceStep because it may be the wizard's first step.
func sourceUpdateConcreteStep(p SourceUpdatePrompt) components.Step {
	return components.Step{
		Name: stepSourceUpd,
		Model: components.NewSelectList(components.NewSelectListParams{
			Description: p.Params.Description,
			Items:       sourceUpdateItems(p.Branch),
		}),
		Summary: sourceUpdateSummary,
	}
}

// sourceUpdateStep builds the optional fast-forward select. It applies only for a
// behind-only source (a fast-forward offer); a diverged or up-to-date source
// auto-skips with a reason (the diverged warning re-appears in the recap).
func sourceUpdateStep(decide func(prev []components.Step) SourceUpdatePrompt) components.Step {
	return components.ChoiceStep(components.ChoiceStepParams{
		Name:    stepSourceUpd,
		Summary: sourceUpdateSummary,
		Decide: func(prev []components.Step) (bool, string, components.NewSelectListParams) {
			p := decide(prev)
			if p.Show && !p.AbortOnDecline {
				return true, "", components.NewSelectListParams{
					Description: p.Params.Description,
					Items:       sourceUpdateItems(p.Branch),
				}
			}
			return false, p.SkipReason, components.NewSelectListParams{}
		},
	})
}

// sourceUpdateSummary labels the fast-forward choice in the completed-step summaries.
func sourceUpdateSummary(model any) string {
	sl, ok := model.(components.SelectListModel)
	if !ok {
		return ""
	}
	if sl.Value() == sourceFastForward {
		return "fast-forward to origin"
	}
	return "keep as-is"
}

// resolveSource returns the source branch: the --from preset, else the picked one.
func resolveSource(steps []components.Step, params WizardParams) string {
	if params.Source != "" {
		return params.Source
	}
	return stepValueByName(steps, stepSourceName)
}

// resolveBranchName returns the worktree's branch: the typed one when the branch
// step runs, else the positional argument that fixed it.
func resolveBranchName(steps []components.Step, params WizardParams) string {
	if !params.IncludeBranch {
		return params.BranchName
	}
	return textValueByName(steps, stepBranchName)
}

// reusesBranch reports whether the worktree will check out an existing local
// branch rather than create one, per the injected Target decider.
func reusesBranch(steps []components.Step, params WizardParams) bool {
	if params.Target == nil {
		return false
	}
	name := resolveBranchName(steps, params)
	if name == "" {
		return false
	}
	return params.Target(name).State == domain.BranchTargetExisting
}

// fastForwardBranch names the branch an accepted fast-forward applies to, asking
// the same decider the step did. Empty when the offer was declined, skipped, or
// never made.
func fastForwardBranch(steps []components.Step, params WizardParams) string {
	if params.SourceUpdate == nil || stepValueByName(steps, stepSourceUpd) != sourceFastForward {
		return ""
	}
	return params.SourceUpdate(SourceUpdateParams{
		Branch: resolveBranchName(steps, params),
		Source: resolveSource(steps, params),
	}).Branch
}

// resolveEnv returns the env override: the --env-from preset, else the picked one.
func resolveEnv(steps []components.Step, params WizardParams) string {
	if !params.IncludeEnv {
		return params.EnvOverride
	}
	return stepValueByName(steps, stepEnvName)
}

// IsolationRecapLine is the isolation line a host wizard adds to its own recap,
// and false for a project with nothing to isolate. A flag that answered the
// step keeps its line: a flag must never make a recap line disappear.
func IsolationRecapLine(steps []components.Step, params WizardParams) (string, bool) {
	if !rules.IsolationRecapShown(rules.IsolationRecapShownParams{Applies: params.IsolationApplies, Override: params.IsolationOverride}) {
		return "", false
	}
	return domain.RecapFieldIsolation + rules.IsolationSummary(resolveIsolation(steps, params)), true
}

func includesIsolation(params WizardParams) bool {
	return params.IsolationApplies && params.IsolationOverride == ""
}

// resolveIsolation is the step's answer, else the flag, else the project's.
func resolveIsolation(steps []components.Step, params WizardParams) domain.Isolation {
	if !includesIsolation(params) {
		return rules.EffectiveIsolation(rules.FirstIsolation(params.IsolationOverride, params.IsolationDefault))
	}
	return components.IsolationAnswer(steps, params.IsolationDefault)
}

// buildCreateRecap recaps the selections with ⚠ lines for a diverged source and
// the env fallback, using the same deciders the steps did.
func buildCreateRecap(prev []components.Step, params WizardParams) string {
	source := resolveSource(prev, params)
	env := resolveEnv(prev, params)
	envLabel := env
	if envLabel == "" {
		envLabel = "config default"
	}

	branchName := resolveBranchName(prev, params)
	reused := reusesBranch(prev, params)
	ffBranch := fastForwardBranch(prev, params)

	branchLabel := branchName
	if reused {
		branchLabel += domain.BranchReusedSuffix
	}

	// The source line is a start-point for a new branch and only the recorded sync
	// parent for a reused one; the fast-forward annotation follows its subject.
	sourceField := domain.RecapFieldSource
	if reused {
		sourceField = domain.RecapFieldParent
	}
	sourceLabel := source
	if ffBranch != "" && ffBranch == source {
		sourceLabel += " (fast-forward to origin)"
	}

	var lines []string
	if branchLabel != "" {
		lines = append(lines, domain.RecapFieldBranch+branchLabel)
	}
	lines = append(lines,
		sourceField+sourceLabel,
		domain.RecapFieldEnv+envLabel,
	)
	if line, shown := IsolationRecapLine(prev, params); shown {
		lines = append(lines, line)
	}
	if ffBranch != "" && ffBranch != source {
		lines = append(lines, fmt.Sprintf(domain.RecapUpdateFastForward, ffBranch))
	}

	if warnings := CreateWarnings(prev, params); len(warnings) > 0 {
		lines = append(lines, "")
		lines = append(lines, warnings...)
	}
	return strings.Join(lines, "\n")
}

// CreateWarnings returns the ⚠ recap lines for the create sub-flow — a diverged
// source and the env fallback — from the same deciders the steps used. Exposed so
// a host wizard (extract) can fold them into its combined recap.
func CreateWarnings(steps []components.Step, params WizardParams) []string {
	source := resolveSource(steps, params)
	env := resolveEnv(steps, params)
	var warnings []string
	if params.SourceUpdate != nil {
		p := params.SourceUpdate(SourceUpdateParams{
			Branch: resolveBranchName(steps, params),
			Source: source,
		})
		if p.Show && p.AbortOnDecline && p.Params.Warning != "" {
			warnings = append(warnings, "⚠ "+p.Params.Warning)
		}
	}
	if params.EnvFallback != nil {
		if show, ep := params.EnvFallback(source, env); show && ep.Warning != "" {
			warnings = append(warnings, "⚠ "+ep.Warning)
		}
	}
	return warnings
}

// extractResult reads the wizard answers, translating the recap's "No, cancel"
// into ErrUserAborted.
func extractResult(final components.WizardModel, params WizardParams) (WizardResult, error) {
	steps := final.Steps()
	if stepValueByName(steps, stepConfirm) == domain.WizardCancelValue {
		return WizardResult{}, domain.ErrUserAborted
	}
	return ReadCreateResult(steps, params), nil
}

func stepIndexByName(steps []components.Step, name string) int {
	for i := range steps {
		if steps[i].Name == name {
			return i
		}
	}
	return -1
}

func stepModelByName(steps []components.Step, name string) any {
	if i := stepIndexByName(steps, name); i >= 0 {
		return steps[i].Model
	}
	return nil
}

func stepValueByName(steps []components.Step, name string) string {
	if sl, ok := stepModelByName(steps, name).(components.SelectListModel); ok {
		return sl.Value()
	}
	return ""
}

func textValueByName(steps []components.Step, name string) string {
	if ti, ok := stepModelByName(steps, name).(components.TextInputModel); ok {
		return ti.Value()
	}
	return ""
}

// envStrategySummary renders the chosen env strategy, labelling the empty
// (config default) choice explicitly.
func envStrategySummary(model any) string {
	child, ok := model.(components.SelectListModel)
	if !ok {
		return ""
	}
	if v := child.Value(); v != "" {
		return v
	}
	return "config default"
}

func buildBranchItems(branches []domain.BranchCandidate, defaultBranch string) []components.SelectItem {
	pinned := ""
	for _, b := range branches {
		if b.Name == defaultBranch {
			pinned = defaultBranch
			break
		}
	}
	return components.BranchItems(components.BranchItemsParams{
		Candidates:   branches,
		Pinned:       pinned,
		PinnedSuffix: domain.PinnedSuffixDefault,
	})
}

func buildEnvItems(strategy domain.EnvStrategy) []components.SelectItem {
	return []components.SelectItem{
		{Label: "Use config default (" + string(strategy) + ")", Value: ""},
		{Label: "example — copy .env.example → .env", Value: string(domain.EnvStrategyExample)},
		{Label: "main — copy .env from main worktree", Value: string(domain.EnvStrategyMain)},
		{Label: "parent — copy .env from source worktree", Value: string(domain.EnvStrategyParent)},
	}
}
