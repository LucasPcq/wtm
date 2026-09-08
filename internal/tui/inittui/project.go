package inittui

import (
	"fmt"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/tui/branchrefresh"
	"github.com/LucasPcq/wtm/internal/tui/components"
)

// Step keys identify wizard steps so extraction is positional-independent —
// the same builders feed both the full init wizard and the targeted
// `wtm init --only <section>` re-init wizard.
const (
	stepBasePath       = "base_path"
	stepBaseBranch     = "base_branch"
	stepEnvGate        = "env_gate"
	stepEnvStrategy    = "env_strategy"
	stepEnvFiles       = "env_files"
	stepHooksGate      = "hooks_gate"
	stepHooks          = "hooks"
	stepHooksCleanGate = "hooks_clean_gate"
	stepHooksClean     = "hooks_clean"
	stepDocker         = "docker"
	stepComposePatch   = "compose_patch"
	stepScriptKinds    = "script_kinds"
	stepPorts          = "ports"
	stepURLs           = "urls"
	stepCmds           = "cmds"
	stepPortRoute      = "port_route"
	stepRuns           = "runs"
	stepEnvLink        = "env_link"
	stepAddressing     = "addressing"
	stepRecap          = "recap"
	stepProfiles       = "profiles"
	stepScripts        = "scripts"
)

// stepSet accumulates wizard steps and records each one's index by key.
type stepSet struct {
	steps []components.Step
	idx   map[string]int
}

func newStepSet() *stepSet {
	return &stepSet{idx: map[string]int{}}
}

func (s *stepSet) add(key string, step components.Step) {
	s.idx[key] = len(s.steps)
	s.steps = append(s.steps, step)
}

// at returns the index of a registered step, or -1 if it was not built.
func (s *stepSet) at(key string) int {
	if i, ok := s.idx[key]; ok {
		return i
	}
	return -1
}

// SectionPrefill carries the current on-disk config so a targeted re-init wizard
// pre-selects what's already configured. A nil prefill (full init) falls back to
// detection-driven defaults with no "current/new" tags.
type SectionPrefill struct {
	BaseBranch    string
	EnvStrategy   string
	EnvTargets    map[string]bool
	OnCreate      []domain.HookCommand
	OnClean       []domain.HookCommand
	DockerFiles   map[string]bool
	ScriptIndices map[int]bool
}

// RunProjectWizard presents the full project init wizard pre-populated with
// detection results. Returns ErrUserAborted if the user aborts.
//
// The wizard is organised by concept. Each optional section (env, hooks) is
// introduced by a "gate" step that explains what the section does and offers
// Configure / Skip; its sub-steps auto-skip when the gate is skipped. Services
// are configured separately by `wtm run init` (see RunServicesWizard).
func RunProjectWizard(projectDir string, detection domain.InitDetectionResult) (domain.InitProjectAnswers, error) {
	s := newStepSet()
	holder := &detection.Branches

	s.add(stepBasePath, basePathStep())
	s.add(stepBaseBranch, baseBranchStep(baseBranchStepParams{
		Holder:      holder,
		Pinned:      detection.BaseBranch,
		Description: "New worktrees branch off this base by default — usually your main development branch.",
	}))

	s.add(stepEnvGate, envGate(detection))
	addEnvSteps(s, detection, autoSkipWhenGateSkipped(s.at(stepEnvGate)), nil)

	s.add(stepHooksGate, hooksGate(detection))
	addHooksSteps(s, detection, autoSkipWhenGateSkipped(s.at(stepHooksGate)), nil)

	s.add(stepHooksCleanGate, hooksCleanGate())
	addHooksCleanSteps(s, autoSkipWhenGateSkipped(s.at(stepHooksCleanGate)), nil)

	final, err := runWizard(runWizardParams{steps: s.steps, projectDir: projectDir, holder: holder})
	if err != nil {
		return domain.InitProjectAnswers{}, err
	}
	return extractProjectAnswers(final, detection, s.idx), nil
}

// RunServicesWizard presents the services-only wizard used by `wtm run init`.
// The user has already opted into the run module by invoking the command, so
// there is no Configure/Skip gate: it goes straight to the docker-compose and
// package-script multiselects. A nil prefill (fresh init) pre-selects the
// detection defaults; a non-nil prefill (re-run) pre-selects what run.toml
// already declares, so what stays checked is kept and what is unchecked is
// dropped. Returns empty answers with no error when nothing is detected — the
// caller reports how to add jobs manually.
func RunServicesWizard(params ServicesWizardParams) (domain.InitProjectAnswers, error) {
	s := newStepSet()
	steps := addServicesSteps(s, addServicesStepsParams{
		Detection:    params.Detection,
		Existing:     params.Existing,
		Prefill:      params.Prefill,
		PatchCompose: params.PatchCompose,
		EnvScans:     params.EnvScans,
		EnvLines:     params.EnvLines,
		EnvFiles:     params.EnvFiles,
	})

	if len(s.steps) == 0 {
		return domain.InitProjectAnswers{}, nil
	}
	s.add(stepRecap, servicesRecapStep(servicesRecapParams{
		Steps:   steps,
		Cmds:    s.at(stepCmds),
		Answers: []int{s.at(stepEnvLink), s.at(stepComposePatch)},
	}))

	final, err := runWizard(runWizardParams{steps: s.steps, projectDir: params.ProjectDir})
	if err != nil {
		return domain.InitProjectAnswers{}, err
	}
	if recapCancelled(final, s.at(stepRecap)) {
		return domain.InitProjectAnswers{}, domain.ErrUserAborted
	}

	answers := extractProjectAnswers(final, params.Detection, s.idx)
	answers.PatchCompose = answers.PatchCompose || params.PatchCompose
	return answers, nil
}

// ServicesWizardParams holds the inputs for RunServicesWizard.
type ServicesWizardParams struct {
	ProjectDir string
	Detection  domain.InitDetectionResult
	Existing   domain.RunConfig
	Prefill    *SectionPrefill
	// PatchCompose is --patch-compose already given on the command line: the
	// rewrite is authorized, so the wizard states it instead of asking again.
	PatchCompose bool
	// EnvScans feeds the same resolution the recap runs, so the rewrite step
	// cannot announce a patch a .env port later withdraws.
	EnvScans map[string]domain.EnvPortScan
	// EnvLines is the project's env files read once, so the link step can match
	// them against a config still being composed.
	EnvLines map[string][]domain.EnvLine
	// EnvFiles are the value targets the project provisions, which is what says
	// where a port key would be written — and whether that file exists as a
	// target at all.
	EnvFiles []domain.EnvFile
}

// SectionWizardParams holds inputs for RunSectionWizard.
type SectionWizardParams struct {
	ProjectDir string
	Sections   []string
	Detection  domain.InitDetectionResult
	Prefill    *SectionPrefill
	// Confirm, when set, appends a final confirmation step so the re-init prompt
	// lives inside the wizard (breadcrumb + back) instead of as an orphaned prompt
	// after it. Declining it — or Esc at the first step — yields ErrUserAborted.
	Confirm *components.NewConfirmParams
}

// RunSectionWizard presents a targeted wizard for the requested sections only,
// without gates or core steps. Used by `wtm init --only <section>`. Returns a
// nil-ish answers set with no steps when nothing is configurable; callers handle
// the empty case.
func RunSectionWizard(params SectionWizardParams) (domain.InitProjectAnswers, error) {
	s := newStepSet()
	holder := &params.Detection.Branches
	for _, section := range params.Sections {
		switch section {
		case domain.SectionWorktrees:
			addWorktreesSteps(s, holder, params.Detection, params.Prefill)
		case domain.SectionEnv:
			addEnvSteps(s, params.Detection, nil, params.Prefill)
		case domain.SectionHooks:
			addHooksSteps(s, params.Detection, nil, params.Prefill)
			addHooksCleanSteps(s, nil, params.Prefill)
		}
	}

	if len(s.steps) == 0 {
		return domain.InitProjectAnswers{}, nil
	}

	steps := s.steps
	if params.Confirm != nil {
		steps = append(steps, reinitConfirmStep(*params.Confirm))
	}

	// Only wire the background branch refresh when a base-branch step is present.
	wp := runWizardParams{steps: steps, projectDir: params.ProjectDir}
	if s.at(stepBaseBranch) >= 0 {
		wp.holder = holder
	}

	final, err := runWizard(wp)
	if err != nil {
		return domain.InitProjectAnswers{}, err
	}
	if params.Confirm != nil {
		finalSteps := final.Steps()
		if sl, ok := finalSteps[len(finalSteps)-1].Model.(components.SelectListModel); ok && sl.Value() == domain.WizardCancelValue {
			return domain.InitProjectAnswers{}, domain.ErrUserAborted
		}
	}
	return extractProjectAnswers(final, params.Detection, s.idx), nil
}

// reinitConfirmValue is the recap step's action value for a targeted re-init.
const reinitConfirmValue = "reinit"

// reinitConfirmStep builds the final recap for `wtm init --only`: it restates the
// values chosen on the section steps (so the user sees exactly what will change),
// folds the warning into a ⚠ line, and offers "Yes, re-initialize" then the
// constant "No, cancel". It is always the last step (section steps precede it), so
// Build always runs on entry.
func reinitConfirmStep(p components.NewConfirmParams) components.Step {
	return components.RecapStep(components.RecapStepParams{
		Name: "Confirm",
		Build: func(prev []components.Step) components.RecapContent {
			var lines []string
			if p.Description != "" {
				lines = append(lines, p.Description, "")
			}
			for _, s := range prev {
				if s.Summary == nil {
					continue
				}
				if v := s.Summary(s.Model); v != "" {
					lines = append(lines, "• "+s.Name+": "+v)
				}
			}
			if p.Warning != "" {
				lines = append(lines, "", "⚠ "+p.Warning)
			}
			return components.RecapContent{
				Description: strings.Join(lines, "\n"),
				Actions: []components.SelectItem{
					{Label: "Yes, re-initialize", Value: reinitConfirmValue},
				},
			}
		},
	})
}

// servicesRecapStep restates what the run is about to write and warns about what
// will still collide. It is the point the flow was missing: every answer visible
// at once, before anything is written, with a way back to the step that set it.
// servicesSteps are the two readings the recap needs: the config as it will
// land on disk, and the jobs the unchecking is about to drop from it — the
// second being invisible in the first, which is precisely why it is shown.
type servicesSteps struct {
	Written func([]components.Step) domain.RunConfig
	Removed func([]components.Step) []string
}

type servicesRecapParams struct {
	Steps servicesSteps
	Cmds  int
	// Answers are the steps whose outcome no config field carries — the yes/no
	// ones — listed under their own heading rather than mixed into the content.
	Answers []int
}

func servicesRecapStep(params servicesRecapParams) components.Step {
	return components.RecapStep(components.RecapStepParams{
		Name: domain.RecapStepName,
		Build: func(prev []components.Step) components.RecapContent {
			cfg := params.Steps.Written(prev)

			lines := []string{"", domain.RecapStepIntro}
			lines = appendSection(lines, domain.RecapJobsTitle, rules.RecapJobLines(cfg))
			lines = appendSection(lines, domain.RecapRemovedTitle, params.Steps.Removed(prev))
			lines = appendSection(lines, domain.RecapProfilesTitle, rules.RecapProfileLines(cfg))
			lines = appendSection(lines, domain.RecapAnswersTitle, answerLines(prev, params.Answers))

			for _, warning := range recapWarnings(cfg, prev, params.Cmds) {
				lines = append(lines, "", warning)
			}

			return components.RecapContent{
				Description: strings.Join(lines, "\n"),
				Actions:     []components.SelectItem{{Label: domain.RecapWriteLabel, Value: domain.RecapWriteValue}},
			}
		},
	})
}

// appendSection separates each group with a blank line: run together, the jobs,
// the profiles and the answers read as one undifferentiated list.
func appendSection(lines []string, title string, rows []string) []string {
	if len(rows) == 0 {
		return lines
	}
	lines = append(lines, "", title)
	for _, row := range rows {
		lines = append(lines, domain.RecapRowIndent+row)
	}
	return lines
}

// answerLines restates the yes/no steps, an unasked one included: what was not
// asked is as much a part of the outcome as what was.
func answerLines(prev []components.Step, at []int) []string {
	width := 0
	for _, i := range at {
		if i >= 0 && i < len(prev) {
			width = max(width, len([]rune(prev[i].Name)))
		}
	}

	var lines []string
	for _, i := range at {
		if i < 0 || i >= len(prev) {
			continue
		}
		step := prev[i]
		answer := domain.RecapNotAsked
		if step.Summary != nil {
			if summary := step.Summary(step.Model); summary != "" {
				answer = summary
			}
		}
		lines = append(lines, fmt.Sprintf(domain.RecapJobLineFmt, rules.Pad(step.Name, width), answer))
	}
	return lines
}

func recapWarnings(cfg domain.RunConfig, prev []components.Step, cmds int) []string {
	var warnings []string
	if undeclared := rules.ServicesWithoutPorts(cfg); len(undeclared) > 0 {
		warnings = append(warnings,
			fmt.Sprintf(domain.RecapUndeclaredWarnFmt, strings.Join(undeclared, domain.CmdListVarSep)))
	}
	if ignoring := jobsIgnoringTheirPort(prev, cmds); len(ignoring) > 0 {
		warnings = append(warnings,
			fmt.Sprintf(domain.RecapIgnoredPortWarnFmt, strings.Join(ignoring, domain.CmdListVarSep)))
	}
	return warnings
}

// jobsIgnoringTheirPort names the commands the user chose to leave as they are.
// wtm cannot know whether they read the variable on their own, so the recap
// states it rather than deciding for them.
func jobsIgnoringTheirPort(prev []components.Step, at int) []string {
	if at < 0 || at >= len(prev) {
		return nil
	}
	cl, ok := prev[at].Model.(components.CmdListModel)
	if !ok {
		return nil
	}
	var jobs []string
	for _, fix := range cl.Fixes() {
		if rules.CmdMissesItsPort(fix) {
			jobs = append(jobs, fix.Job)
		}
	}
	return jobs
}

func recapCancelled(final components.WizardModel, at int) bool {
	steps := final.Steps()
	if at < 0 || at >= len(steps) {
		return false
	}
	sl, ok := steps[at].Model.(components.SelectListModel)
	return ok && sl.Value() == domain.WizardCancelValue
}

// runWizardParams holds inputs for runWizard. When holder is non-nil the wizard
// fetches origin in the background on open and wires the `r` refresh, keeping the
// base-branch divergence badges up to date.
type runWizardParams struct {
	steps      []components.Step
	projectDir string
	holder     *[]domain.BranchCandidate
}

// runWizard runs the bubbletea program for the given steps and returns the
// final model, mapping abort/quit to ErrUserAborted.
func runWizard(params runWizardParams) (components.WizardModel, error) {
	wp := components.WizardParams{Steps: params.steps}
	if params.holder != nil {
		wp.InitCmd = branchrefresh.Cmd(params.projectDir)
		wp.Loading = true
		wp.LoadingText = domain.LoadingBranchesText
		wp.OnMsg = branchrefresh.Handler(params.projectDir, params.holder)
	}

	wiz := components.NewWizardWithParams(wp)
	finalModel, err := tea.NewProgram(wiz).Run()
	if err != nil {
		return components.WizardModel{}, fmt.Errorf("wizard: %w", err)
	}
	final, ok := finalModel.(components.WizardModel)
	if !ok || final.Aborted() {
		return components.WizardModel{}, domain.ErrUserAborted
	}
	return final, nil
}

// ── Step builders ───────────────────────────────────────────────────────────

func basePathStep() components.Step {
	return components.Step{
		Name: "Worktree directory",
		Model: components.NewTextInput(components.NewTextInputParams{
			Title:       "Worktree directory",
			Description: "wtm creates each worktree in this directory, relative to the repo root. Keeping it outside the repo avoids cluttering your main checkout.",
			Placeholder: domain.DefaultBasePath,
		}),
		Summary: textInputSummary,
		Callout: true,
	}
}

// baseBranchStepParams holds inputs for baseBranchStep.
type baseBranchStepParams struct {
	Holder      *[]domain.BranchCandidate
	Pinned      string
	Description string
}

// baseBranchStep builds the base-branch picker step. It reads candidates from the
// shared holder via Build so a background origin fetch (CanRefresh / the `r` key)
// updates the divergence badges in place.
func baseBranchStep(params baseBranchStepParams) components.Step {
	build := func() any {
		return components.NewSelectList(components.NewSelectListParams{
			Title:       "Base branch",
			Description: params.Description,
			Items:       baseBranchItems(*params.Holder, params.Pinned),
		})
	}
	return components.Step{
		Name:       "Base branch",
		Model:      build(),
		Build:      func([]components.Step) any { return build() },
		CanRefresh: true,
		Summary:    selectListSummary,
		Callout:    true,
	}
}

// baseBranchItems builds the base-branch picker rows with the detected default
// pinned first and remote-tracking branches grouped after a separator.
func baseBranchItems(branches []domain.BranchCandidate, detected string) []components.SelectItem {
	return components.BranchItems(components.BranchItemsParams{
		Candidates:   branches,
		Pinned:       detected,
		PinnedSuffix: domain.PinnedSuffixDetected,
	})
}

// addWorktreesSteps adds the editable worktrees step (base branch only) for a
// targeted re-init. base_path is intentionally not editable here — changing it
// would orphan existing worktrees (tracked separately).
func addWorktreesSteps(s *stepSet, holder *[]domain.BranchCandidate, detection domain.InitDetectionResult, prefill *SectionPrefill) {
	pinned := detection.BaseBranch
	if prefill != nil && prefill.BaseBranch != "" {
		pinned = prefill.BaseBranch
	}
	s.add(stepBaseBranch, baseBranchStep(baseBranchStepParams{
		Holder:      holder,
		Pinned:      pinned,
		Description: "Default branch new worktrees are created from. Changing it only affects future worktrees.",
	}))
}

func envGate(detection domain.InitDetectionResult) components.Step {
	return sectionGate(sectionGateParams{
		Name: "Environment files",
		Description: "Each new worktree is a clean checkout — your local .env files aren't carried over. " +
			"wtm can copy them in automatically so the worktree runs without redoing your local setup.",
		Detected:       detectedEnv(detection),
		ConfigureLabel: "Configure .env copying",
		SkipLabel:      "Skip — I'll handle .env myself",
	})
}

func addEnvSteps(s *stepSet, detection domain.InitDetectionResult, autoSkip func(components.WizardModel) bool, prefill *SectionPrefill) {
	strategyItems := []components.SelectItem{
		{Label: "example — copy .env.example → .env", Value: string(domain.EnvStrategyExample)},
		{Label: "main — copy .env from main worktree", Value: string(domain.EnvStrategyMain)},
		{Label: "parent — copy .env from source worktree", Value: string(domain.EnvStrategyParent)},
	}
	if prefill != nil {
		strategyItems = moveToFront(strategyItems, prefill.EnvStrategy)
	}
	s.add(stepEnvStrategy, components.Step{
		Name: "Env strategy",
		Model: components.NewSelectList(components.NewSelectListParams{
			Title:       "Env strategy",
			Description: "How wtm provisions .env files in a new worktree: copy .env.example, copy from your main worktree, or from the worktree you branched from.",
			Items:       strategyItems,
		}),
		Summary:  selectListSummary,
		AutoSkip: autoSkip,
		Callout:  true,
	})

	if len(detection.EnvFiles) == 0 {
		return
	}
	items := make([]components.MultiSelectItem, 0, len(detection.EnvFiles))
	for _, f := range detection.EnvFiles {
		selected := prefillSelected(prefill, prefill != nil && prefill.EnvTargets[f.Target], true)
		items = append(items, components.MultiSelectItem{Label: envFileLabel(f), Value: f.Target, Selected: selected})
	}
	s.add(stepEnvFiles, components.Step{
		Name: "Env files",
		Model: components.NewMultiSelect(components.NewMultiSelectParams{
			Title:       "Env files to copy",
			Description: "Which of the detected .env files wtm should copy into every new worktree.",
			Items:       items,
		}),
		Summary:  multiSelectSummary,
		AutoSkip: autoSkip,
		Callout:  true,
	})
}

func hooksGate(detection domain.InitDetectionResult) components.Step {
	return sectionGate(sectionGateParams{
		Name: "Post-create hooks",
		Description: "Run commands automatically right after a worktree is created — typically installing " +
			"dependencies — so it's ready to use immediately instead of needing manual steps.",
		Detected:       detectedHooks(detection),
		ConfigureLabel: "Configure setup commands",
		SkipLabel:      "Skip — no automatic commands",
	})
}

func addHooksSteps(s *stepSet, detection domain.InitDetectionResult, autoSkip func(components.WizardModel) bool, prefill *SectionPrefill) {
	var hooks []domain.HookCommand
	if prefill != nil {
		hooks = prefill.OnCreate
	} else if detection.InstallCommand != "" {
		hooks = append(hooks, domain.HookCommand{Cmd: detection.InstallCommand})
		for _, pkg := range detection.MonorepoPackages {
			hooks = append(hooks, domain.HookCommand{Cmd: detection.InstallCommand, Cwd: pkg})
		}
	}

	s.add(stepHooks, components.Step{
		Name: "Post-create hooks",
		Model: components.NewHookList(components.NewHookListParams{
			Title:       "Post-create hooks",
			Description: "Commands run after creating a worktree. Add, edit, remove or reorder them — then select Done.",
			Hooks:       hooks,
		}),
		Summary:  hookListSummary,
		AutoSkip: autoSkip,
		Callout:  true,
	})
}

func hooksCleanGate() components.Step {
	return sectionGate(sectionGateParams{
		Name: "Pre-clean hooks",
		Description: "Run commands automatically right before a worktree is removed — typically tearing down " +
			"external resources like Docker — so nothing is left orphaned after cleanup.",
		ConfigureLabel: "Configure teardown commands",
		SkipLabel:      "Skip — no teardown commands",
	})
}

// addHooksCleanSteps adds the on_clean hook-list step. Unlike post-create hooks
// there is nothing to auto-detect, so the list starts empty (or from the prefill
// on re-init).
func addHooksCleanSteps(s *stepSet, autoSkip func(components.WizardModel) bool, prefill *SectionPrefill) {
	var hooks []domain.HookCommand
	if prefill != nil {
		hooks = prefill.OnClean
	}

	s.add(stepHooksClean, components.Step{
		Name: "Pre-clean hooks",
		Model: components.NewHookList(components.NewHookListParams{
			Title:       "Pre-clean hooks",
			Description: "Commands run before removing a worktree (e.g. `docker compose down`). Add, edit, remove or reorder them — then select Done.",
			Hooks:       hooks,
		}),
		Summary:  hookListSummary,
		AutoSkip: autoSkip,
		Callout:  true,
	})
}

// addScriptKindStep only appears when a script was checked outside the dev ones.
// The kind decides whether a job blocks its profile, and the name gets it wrong
// in both directions: `preview` serves requests while `start` is production.
func addScriptKindStep(s *stepSet, params addServicesStepsParams) {
	detection := params.Detection
	scripts := s.at(stepScripts)
	if scripts < 0 {
		return
	}

	skipReason := ""
	s.add(stepScriptKinds, components.Step{
		Name: domain.ScriptKindStepName,
		Build: func(prev []components.Step) any {
			return components.NewKindList(components.NewKindListParams{
				Title:       domain.ScriptKindStepTitle,
				Description: domain.ScriptKindStepDesc,
				Entries:     scriptKindChoices(scriptKindChoicesParams{Prev: prev, Scripts: scripts, Params: params}),
			})
		},
		AutoSkip: func(w components.WizardModel) bool {
			asked := len(scriptKindChoices(scriptKindChoicesParams{Prev: w.Steps(), Scripts: scripts, Params: params})) == 0
			if asked {
				skipReason = rules.ScriptKindsSkipReason(len(selectedScripts(w.Steps(), scripts, detection.PackageScripts)))
			}
			return asked
		},
		SkipReason: func() string { return skipReason },
		Summary:    kindListSummary,
		Callout:    true,
	})
}

type scriptKindChoicesParams struct {
	Prev    []components.Step
	Scripts int
	Params  addServicesStepsParams
}

// scriptKindChoices are the checked scripts the name does not settle — the ones
// the wizard pre-checked need no question. The label carries the package: two
// workspaces both declaring "build" are two separate answers.
func scriptKindChoices(p scriptKindChoicesParams) []domain.JobKindChoice {
	detected := p.Params.Detection.PackageScripts

	var choices []domain.JobKindChoice
	for _, script := range selectedScripts(p.Prev, p.Scripts, detected) {
		if rules.PreselectScript(rules.PreselectScriptParams{Script: script, All: detected}) {
			continue
		}
		choices = append(choices, domain.JobKindChoice{
			Label:     scriptLabel(script),
			Cmd:       script.Cmd,
			Name:      script.Name,
			Workspace: script.Workspace,
			Kind: rules.ProposedScriptKind(rules.ProposedScriptKindParams{
				Script:         script,
				Config:         p.Params.Existing,
				PackageManager: p.Params.Detection.PackageManager,
			}),
		})
	}
	return choices
}

// scriptLabel names a script by its package, the way the selection step does.
func scriptLabel(script domain.PackageScript) string {
	return fmt.Sprintf(domain.ScriptLabelFmt, scriptScope(script), script.Name)
}

func scriptScope(script domain.PackageScript) string {
	if script.Workspace == "" {
		return domain.ScriptScopeRoot
	}
	return script.Workspace
}

func kindListSummary(model any) string {
	kl, ok := model.(components.KindListModel)
	if !ok {
		return ""
	}
	if len(kl.Entries()) == 0 {
		return domain.RecapNotAsked
	}
	services := 0
	for _, entry := range kl.Entries() {
		if entry.Kind == domain.JobKindService {
			services++
		}
	}
	return fmt.Sprintf(domain.KindListSummaryFmt, services, len(kl.Entries())-services)
}

// addPortsAndProfilesSteps turns the selection into a configuration: the ports
// detection pre-filled, then the split `run up` will offer. Both read the live
// selections, so both are declared after the steps they read.
func addPortsAndProfilesSteps(s *stepSet, params addServicesStepsParams) (steps servicesSteps) {
	docker, scripts := s.at(stepDocker), s.at(stepScripts)
	detection := params.Detection

	resolved := func(prev []components.Step) rules.DetectedPortsOutcome {
		answers := answersFromSteps(answersFromStepsParams{
			Prev: prev, Docker: docker, Scripts: scripts, Detection: detection,
		})
		answers.SelectionAsked = true
		return rules.ResolveDetectedPorts(rules.ResolveDetectedPortsParams{
			Answers:        answers,
			PackageManager: detection.PackageManager,
			Existing:       params.Existing,
			Deselected: rules.DeselectedJobs(rules.DeselectedJobsParams{
				Existing:             params.Existing,
				PackageManager:       detection.PackageManager,
				DetectedScripts:      detection.PackageScripts,
				SelectedScripts:      answers.SelectedPackageScripts,
				DetectedComposeFiles: detection.DockerComposeFiles,
				SelectedComposeFiles: answers.DockerComposeFiles,
				Asked:                answers.SelectionAsked,
			}),
			Plan: rules.PlanComposePorts(rules.PlanComposePortsParams{
				Scans: detection.ComposeScans,
				Files: answers.DockerComposeFiles,
				Patch: true,
			}),
			EnvScansByDir: params.EnvScans,
		})
	}

	// Declared before the ports step, which reads its answer: a service whose
	// children hold the ports is not a service that forgot to declare one.
	s.add(stepRuns, components.Step{
		Name: domain.RunnerListStepName,
		Build: func(prev []components.Step) any {
			return components.NewRunnerList(components.NewRunnerListParams{
				Title:       domain.RunnerListStepTitle,
				Description: domain.RunnerListStepDesc,
				Choices:     runnerChoicesFor(resolved(prev).Config, prev, docker),
			})
		},
		AutoSkip: func(w components.WizardModel) bool {
			return len(runnerChoicesFor(resolved(w.Steps()).Config, w.Steps(), docker)) == 0
		},
		SkipReason: func() string { return domain.SkipReasonNoRunnerCandidate },
		Summary:    runnerListSummary,
		Callout:    true,
	})

	runners := s.at(stepRuns)
	withRunners := func(prev []components.Step) domain.RunConfig {
		return rules.ApplyRunnerChoices(rules.ApplyRunnerChoicesParams{Config: resolved(prev).Config, Choices: runnerChoicesOf(prev, runners)})
	}

	portsSkipReason := ""
	s.add(stepPorts, components.Step{
		Name: domain.PortListStepName,
		Build: func(prev []components.Step) any {
			return components.NewPortList(components.NewPortListParams{
				Title:       domain.PortListStepTitle,
				Description: domain.PortListStepDesc,
				Entries:     portEntriesFor(withRunners(prev), prev, docker),
			})
		},
		AutoSkip: func(w components.WizardModel) bool {
			cfg := withRunners(w.Steps())
			if len(portEntriesFor(cfg, w.Steps(), docker)) > 0 {
				return false
			}
			portsSkipReason = rules.PortsSkipReason(cfg)
			return true
		},
		SkipReason: func() string { return portsSkipReason },
		Summary:    portListSummary,
		Callout:    true,
	})

	ports := s.at(stepPorts)
	settled := func(prev []components.Step) domain.RunConfig {
		return rules.ApplyInitAnswers(rules.ApplyInitAnswersParams{
			Config: withRunners(prev),
			Ports:  portEntriesOf(prev, ports),
		})
	}
	// written is settled plus the answers the later steps add, which is what the
	// recap has to show: the config as it will land on disk, not a stage of it.
	steps.Removed = func(prev []components.Step) []string { return resolved(prev).Removed }
	steps.Written = func(prev []components.Step) domain.RunConfig {
		outcome := resolved(prev)
		return rules.ApplyInitAnswers(rules.ApplyInitAnswersParams{
			Config:        withRunners(prev),
			Ports:         portEntriesOf(prev, ports),
			Cmds:          cmdFixesOf(prev, s.at(stepCmds)),
			Profiles:      profilesOf(prev, s.at(stepProfiles)),
			ProfilesAsked: profileStepAnswered(prev, s.at(stepProfiles)),
			URLs:          urlAnswerOf(prev, s.at(stepURLs)),
			URLsAsked:     urlStepAnswered(prev, s.at(stepURLs)),
			NewJobs:       outcome.Merge.Added,
		})
	}

	cmdSkipReason := ""
	// Declared before the commands step, which it narrows: a job routed to its
	// own .env has nothing left to fix on its command line.
	s.add(stepPortRoute, components.Step{
		Name: domain.RouteListStepName,
		Build: func(prev []components.Step) any {
			return components.NewRouteList(components.NewRouteListParams{
				Title:       domain.RouteListStepTitle,
				Description: domain.RouteListStepDesc,
				Rows:        portRouteRows(portRouteParams{Config: settled(prev), Steps: prev, Docker: docker, EnvScans: params.EnvScans, EnvFiles: params.EnvFiles}),
			})
		},
		AutoSkip: func(w components.WizardModel) bool {
			return len(portRouteRows(portRouteParams{Config: settled(w.Steps()), Steps: w.Steps(), Docker: docker, EnvScans: params.EnvScans, EnvFiles: params.EnvFiles})) == 0
		},
		SkipReason: func() string { return domain.SkipReasonNoPortedJob },
		Summary:    routeListSummary,
		Callout:    true,
	})

	s.add(stepCmds, components.Step{
		Name: domain.CmdListStepName,
		Build: func(prev []components.Step) any {
			return components.NewCmdList(components.NewCmdListParams{
				Title:       domain.CmdListStepTitle,
				Description: domain.CmdListStepDesc,
				Fixes:       cmdFixesFor(cmdFixesParams{Config: settled(prev), Steps: prev, Docker: docker, EnvScans: params.EnvScans, Routes: portRoutesOf(prev, s.at(stepPortRoute))}),
			})
		},
		AutoSkip: func(w components.WizardModel) bool {
			if len(cmdFixesFor(cmdFixesParams{Config: settled(w.Steps()), Steps: w.Steps(), Docker: docker, EnvScans: params.EnvScans, Routes: portRoutesOf(w.Steps(), s.at(stepPortRoute))})) > 0 {
				return false
			}
			cmdSkipReason = domain.SkipReasonCommandsRead
			return true
		},
		SkipReason: func() string { return cmdSkipReason },
		Summary:    cmdListSummary,
		Callout:    true,
	})

	// Declared after the commands step: a job whose command was just shown to
	// ignore its port is one the reader can now decline to publish knowingly.
	s.add(stepURLs, components.Step{
		Name: domain.URLListStepName,
		Build: func(prev []components.Step) any {
			return components.NewMultiSelect(components.NewMultiSelectParams{
				Title:       domain.URLListStepTitle,
				Description: domain.URLListStepDesc,
				Items:       urlItemsFor(settled(prev), resolved(prev).Merge.Added),
			})
		},
		AutoSkip: func(w components.WizardModel) bool {
			return len(urlItemsFor(settled(w.Steps()), resolved(w.Steps()).Merge.Added)) == 0
		},
		SkipReason: func() string { return domain.SkipReasonNoListeningPort },
		Summary:    urlListSummary,
		Callout:    true,
	})

	// Declared right after the urls: the reader has just said which jobs get a
	// name, and this is what those names cost.
	s.add(stepAddressing, components.Step{
		Name: domain.AddressingStepName,
		Build: func(prev []components.Step) any {
			return components.NewSelectList(components.NewSelectListParams{
				Title:       domain.AddressingStepTitle,
				Description: domain.AddressingStepDesc,
				Items:       addressingItems(rules.AddressingChoices(rules.EffectiveAddressing(steps.Written(prev)))),
			})
		},
		AutoSkip: func(w components.WizardModel) bool {
			return !rules.AnyJobPublishesAName(steps.Written(w.Steps()))
		},
		SkipReason: func() string { return domain.SkipReasonNoName },
		Summary:    selectListSummary,
		Callout:    true,
	})

	s.add(stepProfiles, components.Step{
		Name: domain.ProfileListStepName,
		Build: func(prev []components.Step) any {
			return components.NewProfileList(components.NewProfileListParams{
				Title:       domain.ProfileStepTitle,
				Description: domain.ProfileStepDesc,
				Profiles: rules.ProposeProfiles(rules.ProposeProfilesParams{
					Config:   resolved(prev).Config,
					Existing: params.Existing.Profiles,
				}),
			})
		},
		AutoSkip: func(w components.WizardModel) bool {
			return len(rules.ProposeProfiles(rules.ProposeProfilesParams{
				Config:   resolved(w.Steps()).Config,
				Existing: params.Existing.Profiles,
			})) == 0
		},
		SkipReason: func() string { return domain.SkipReasonNoJob },
		Summary:    profileListSummary,
		Callout:    true,
	})
	defer func() { _ = steps }()

	s.add(stepEnvLink, components.ConfirmStep(components.ConfirmStepParams{
		Name:    domain.EnvLinkStepName,
		Callout: true,
		Decide: func(prev []components.Step) (bool, string, components.NewConfirmParams) {
			candidates := envLinkCandidates(settled(prev), params.EnvLines)
			if len(candidates) == 0 {
				return false, domain.SkipReasonNoEnvKeyFollows, components.NewConfirmParams{}
			}
			return true, "", components.NewConfirmParams{
				Title: domain.EnvPortLinkConfirm,
				Description: strings.Join(append(
					[]string{domain.EnvPortLinkDescription, ""},
					rules.EnvPortLinkLines(candidates, rules.EnvPortBases(settled(prev)))...), "\n"),
				DefaultYes: true,
			}
		},
	}))

	return steps
}

// cmdFixesOf and profilesOf read a step back, tolerating one this wizard never
// built.
func cmdFixesOf(prev []components.Step, at int) []domain.JobCmdFix {
	if at < 0 || at >= len(prev) {
		return nil
	}
	cl, ok := prev[at].Model.(components.CmdListModel)
	if !ok {
		return nil
	}
	return cl.Fixes()
}

func profileStepAnswered(prev []components.Step, at int) bool {
	if at < 0 || at >= len(prev) {
		return false
	}
	_, ok := prev[at].Model.(components.ProfileListModel)
	return ok
}

func profilesOf(prev []components.Step, at int) []domain.ProfileConfig {
	if at < 0 || at >= len(prev) {
		return nil
	}
	pl, ok := prev[at].Model.(components.ProfileListModel)
	if !ok {
		return nil
	}
	return pl.Profiles()
}

// portEntriesFor exempts the compose jobs: their `ports:` list is complete, so
// they are the one family the step has nothing more to ask about.
func portEntriesFor(cfg domain.RunConfig, prev []components.Step, docker int) []domain.PortEntry {
	return rules.PortEntriesFor(rules.PortEntriesForParams{
		Config:      cfg,
		ComposeJobs: composeJobsIn(cfg, prev, docker),
	})
}

func composeJobsIn(cfg domain.RunConfig, prev []components.Step, docker int) []string {
	return rules.ComposeJobsFor(rules.ComposeJobsParams{
		Config: cfg,
		Files:  selectedComposeFiles(prev, docker),
	})
}

// portEntriesOf reads back what the ports step settled, so a later step sees the
// port the user just declared rather than the one detection failed to find.
func portEntriesOf(prev []components.Step, at int) []domain.PortEntry {
	if at < 0 || at >= len(prev) {
		return nil
	}
	pl, ok := prev[at].Model.(components.PortListModel)
	if !ok {
		return nil
	}
	return pl.Entries()
}

// urlItemsFor offers every candidate pre-answered yes. Publishing is additive —
// the job keeps its own port either way — so the cost of a wrong default falls
// on the reader unchecking a line, not on a run that fails.
func urlItemsFor(cfg domain.RunConfig, newJobs []string) []components.MultiSelectItem {
	candidates := rules.URLCandidatesFor(rules.URLCandidatesForParams{Config: cfg, NewJobs: newJobs})
	width := 0
	for _, candidate := range candidates {
		width = max(width, len([]rune(candidate.Job)))
	}

	items := make([]components.MultiSelectItem, 0, len(candidates))
	for _, candidate := range candidates {
		items = append(items, components.MultiSelectItem{
			Label:    fmt.Sprintf(domain.URLListEntryFmt, rules.Pad(candidate.Job, width), candidate.Port),
			Value:    candidate.Job,
			Selected: candidate.Publish,
		})
	}
	return items
}

// urlAnswerOf and urlStepAnswered read the step back as the raw answer it is.
// Which jobs that answer publishes is settled by ApplyInitAnswers, once the
// ports are in: a job with no port yet is not a candidate, and resolving here
// would judge it too early.
func urlAnswerOf(prev []components.Step, at int) []string {
	ms, ok := urlStep(prev, at)
	if !ok {
		return nil
	}
	return ms.Values()
}

func urlStepAnswered(prev []components.Step, at int) bool {
	_, ok := urlStep(prev, at)
	return ok
}

func urlStep(prev []components.Step, at int) (components.MultiSelectModel, bool) {
	if at < 0 || at >= len(prev) {
		return components.MultiSelectModel{}, false
	}
	ms, ok := prev[at].Model.(components.MultiSelectModel)
	return ms, ok
}

// cmdFixesFor exempts two kinds of job. A compose stack reads its ports from
// the file wtm templated, never from the command that starts it; and a job
// whose ports were read from its own .env already reads them, which is why the
// value was there to detect.
func cmdFixesFor(params cmdFixesParams) []domain.JobCmdFix {
	exempt := composeJobsIn(params.Config, params.Steps, params.Docker)

	// Once the route step has answered, the answer outranks the detection: a job
	// whose .env already carries its port but that the reader moved onto its
	// command must be offered that command, or the move has no way to happen.
	if params.Routes != nil {
		exempt = append(exempt, rules.JobsOnEnvRoute(rules.JobsOnEnvRouteParams{Config: params.Config, Routes: params.Routes})...)
		return rules.JobsMissingPortRef(rules.JobsMissingPortRefParams{Config: params.Config, Exempt: exempt})
	}

	exempt = append(exempt, rules.JobsReadingTheirEnv(rules.JobsReadingTheirEnvParams{
		Config:     params.Config,
		ScansByDir: params.EnvScans,
	})...)
	return rules.JobsMissingPortRef(rules.JobsMissingPortRefParams{Config: params.Config, Exempt: exempt})
}

type cmdFixesParams struct {
	Config   domain.RunConfig
	Steps    []components.Step
	Docker   int
	EnvScans map[string]domain.EnvPortScan
	// Routes is the answer of the step before this one: a job whose every port
	// is read from its own .env has nothing to fix on its command line.
	Routes map[domain.PortRef]domain.PortRoute
}

type portRouteParams struct {
	Config   domain.RunConfig
	Steps    []components.Step
	Docker   int
	EnvScans map[string]domain.EnvPortScan
	EnvFiles []domain.EnvFile
}

// portRouteRows lists every service declaring a port, compose stacks excepted —
// they read theirs from the file wtm templated. The complete list is the point:
// a re-init shows what each job settled on, pre-filled, rather than only what is
// still unresolved.
func portRouteRows(params portRouteParams) []domain.PortRouteRow {
	return rules.PortRouteRows(rules.PortRouteRowsParams{
		Config:      params.Config,
		ComposeJobs: composeJobsIn(params.Config, params.Steps, params.Docker),
		ScansByDir:  params.EnvScans,
		EnvFiles:    params.EnvFiles,
	})
}

func runnerChoicesFor(cfg domain.RunConfig, prev []components.Step, docker int) []domain.JobRunnerChoice {
	return rules.RunnerChoices(rules.RunnerChoicesParams{
		Config:      cfg,
		ComposeJobs: composeJobsIn(cfg, prev, docker),
	})
}

func runnerChoicesOf(prev []components.Step, at int) []domain.JobRunnerChoice {
	if at < 0 || at >= len(prev) {
		return nil
	}
	rl, ok := prev[at].Model.(components.RunnerListModel)
	if !ok {
		return nil
	}
	return rl.Choices()
}

func runnerListSummary(model any) string {
	rl, ok := model.(components.RunnerListModel)
	if !ok {
		return ""
	}
	if len(rl.Choices()) == 0 {
		return domain.RecapNotAsked
	}
	attached := 0
	for _, choice := range rl.Choices() {
		if len(choice.Runners) > 0 {
			attached++
		}
	}
	return fmt.Sprintf(domain.RunnerListSummaryFmt, attached, len(rl.Choices()))
}

// addressingItems renders the two modes the rule ordered. Which one comes
// first is a decision, and it is made in rules/.
func addressingItems(modes []domain.Addressing) []components.SelectItem {
	items := make([]components.SelectItem, 0, len(modes))
	for _, mode := range modes {
		items = append(items, components.SelectItem{Label: rules.AddressingLabel(mode), Value: string(mode)})
	}
	return items
}

func portRoutesOf(prev []components.Step, at int) map[domain.PortRef]domain.PortRoute {
	if at < 0 || at >= len(prev) {
		return nil
	}
	rl, ok := prev[at].Model.(components.RouteListModel)
	if !ok {
		return nil
	}
	return rl.Routes()
}

func routeListSummary(model any) string {
	rl, ok := model.(components.RouteListModel)
	if !ok {
		return ""
	}
	if len(rl.Rows()) == 0 {
		return domain.RecapNotAsked
	}
	env := 0
	for _, row := range rl.Rows() {
		if row.Route == domain.PortRouteEnv {
			env++
		}
	}
	return fmt.Sprintf(domain.RouteListSummaryFmt, env, len(rl.Rows())-env)
}

func selectedComposeFiles(prev []components.Step, docker int) []string {
	if docker < 0 || docker >= len(prev) {
		return nil
	}
	selected, ok := prev[docker].Model.(components.MultiSelectModel)
	if !ok {
		return nil
	}
	return selected.Values()
}

func envLinkCandidates(cfg domain.RunConfig, lines map[string][]domain.EnvLine) []domain.EnvPortLink {
	return rules.EnvPortCandidates(rules.EnvPortCandidatesParams{
		Lines:     lines,
		Bases:     rules.EnvPortBases(cfg),
		Existing:  cfg.EnvPorts,
		JobsByDir: rules.JobsByCwd(cfg),
	})
}

func cmdListSummary(model any) string {
	cl, ok := model.(components.CmdListModel)
	if !ok {
		return ""
	}
	if len(cl.Fixes()) == 0 {
		return domain.RecapNotAsked
	}
	fixed := 0
	for _, fix := range cl.Fixes() {
		if !rules.CmdMissesItsPort(fix) {
			fixed++
		}
	}
	return fmt.Sprintf(domain.CmdListSummaryFmt, fixed, len(cl.Fixes()))
}

func portListSummary(model any) string {
	pl, ok := model.(components.PortListModel)
	if !ok {
		return ""
	}
	declared, answered := 0, 0
	for _, entry := range pl.Entries() {
		switch {
		case entry.Base > 0:
			declared++
		case entry.BindsNone:
			answered++
		}
	}
	if undeclared := len(pl.Entries()) - declared - answered; undeclared > 0 {
		return fmt.Sprintf(domain.PortListSummaryUndeclaredFmt, declared, undeclared)
	}
	return fmt.Sprintf(domain.PortListSummaryFmt, declared)
}

func profileListSummary(model any) string {
	pl, ok := model.(components.ProfileListModel)
	if !ok {
		return ""
	}
	if len(pl.Profiles()) == 0 {
		return domain.RecapNotAsked
	}
	return fmt.Sprintf(domain.ProfileListSummaryFmt, len(pl.Profiles()))
}

type scriptItemsParams struct {
	Scripts        []domain.PackageScript
	PackageManager domain.PackageManager
	Prefill        *SectionPrefill
}

// scriptItems proposes every script and checks the fewest: a job nobody checked
// is a job that is never written, which is what stops the init from producing
// an inventory instead of a configuration.
func scriptItems(params scriptItemsParams) []components.MultiSelectItem {
	pm := string(params.PackageManager)
	width := 0
	for _, script := range params.Scripts {
		width = max(width, len([]rune(scriptScope(script)+domain.ScriptLabelSep+script.Name)))
	}

	items := make([]components.MultiSelectItem, 0, len(params.Scripts))
	for i, script := range params.Scripts {
		name := rules.Pad(scriptScope(script)+domain.ScriptLabelSep+script.Name, width)
		items = append(items, components.MultiSelectItem{
			Label: fmt.Sprintf(domain.ScriptItemLabelFmt, name, pm, script.Name),
			Value: strconv.Itoa(i),
			Selected: prefillSelected(params.Prefill,
				params.Prefill != nil && params.Prefill.ScriptIndices[i],
				rules.PreselectScript(rules.PreselectScriptParams{Script: script, All: params.Scripts})),
		})
	}
	return items
}

type addServicesStepsParams struct {
	Detection    domain.InitDetectionResult
	Existing     domain.RunConfig
	Prefill      *SectionPrefill
	PatchCompose bool
	EnvScans     map[string]domain.EnvPortScan
	EnvLines     map[string][]domain.EnvLine
	EnvFiles     []domain.EnvFile
}

func addServicesSteps(s *stepSet, params addServicesStepsParams) (steps servicesSteps) {
	detection, prefill := params.Detection, params.Prefill
	if len(detection.DockerComposeFiles) > 0 {
		items := make([]components.MultiSelectItem, 0, len(detection.DockerComposeFiles))
		for _, f := range detection.DockerComposeFiles {
			selected := prefillSelected(prefill, prefill != nil && prefill.DockerFiles[f], true)
			items = append(items, components.MultiSelectItem{Label: f, Value: f, Selected: selected})
		}
		s.add(stepDocker, components.Step{
			Name: "Docker services",
			Model: components.NewMultiSelect(components.NewMultiSelectParams{
				Title:       "Docker Compose services",
				Description: "Each selected docker-compose file becomes a service you can start and stop with `wtm run`.",
				Items:       items,
			}),
			Summary: multiSelectSummary,
			Callout: true,
		})
	}

	if len(detection.PackageScripts) > 0 {
		s.add(stepScripts, components.Step{
			Name: domain.ScriptsStepName,
			Model: components.NewMultiSelect(components.NewMultiSelectParams{
				Title:       domain.ScriptsStepTitle,
				Description: rules.ScriptsStepDescription(detection.PackageScripts),
				Items: scriptItems(scriptItemsParams{
					Scripts:        detection.PackageScripts,
					PackageManager: detection.PackageManager,
					Prefill:        prefill,
				}),
			}),
			Summary: packageScriptsSummary,
			Callout: true,
		})
	}

	addScriptKindStep(s, params)

	// Declared last on purpose: the step resolves the ports of both selections,
	// so it must be able to read them — a .env port can withdraw a compose
	// declaration, and the step would otherwise offer a rewrite that never runs.
	steps = addPortsAndProfilesSteps(s, addServicesStepsParams{
		Detection: detection,
		Existing:  params.Existing,
		EnvScans:  params.EnvScans,
		EnvLines:  params.EnvLines,
		EnvFiles:  params.EnvFiles,
	})

	addComposePatchStep(s, addComposePatchStepParams{
		Detection:  detection,
		Existing:   params.Existing,
		Authorized: params.PatchCompose,
		EnvScans:   params.EnvScans,
	})

	return steps
}

// composePatchDescription lays out only the halves that have something to show:
// a file with a literal port and no pinned name asks about ports alone.
func composePatchDescription(patches map[string][]domain.ComposePortBinding, names map[string][]domain.ComposeAbsoluteName) string {
	sections := []string{domain.ComposePatchStepIntro}

	if len(patches) > 0 {
		sections = append(sections, domain.ComposePatchStepPortsLead+"\n"+strings.Join(rules.ComposePatchLines(patches), "\n"))
	}
	if len(names) > 0 {
		sections = append(sections, domain.ComposePatchStepNamesLead+"\n"+strings.Join(rules.ComposeNamePatchLines(names), "\n"))
		if rules.ComposeNamesRenameAVolume(names) {
			sections = append(sections, domain.ComposeNamesVolumeWarning)
		}
	}

	return strings.Join(append(sections, domain.ComposePatchStepEpilogue), "\n\n")
}

// composeNamesFor plans against the wizard's live docker selection, so the
// lines the step asks about are exactly the ones the recap will report.
func composeNamesFor(prev []components.Step, docker int, detection domain.InitDetectionResult) map[string][]domain.ComposeAbsoluteName {
	if docker >= len(prev) {
		return nil
	}
	selected, ok := prev[docker].Model.(components.MultiSelectModel)
	if !ok {
		return nil
	}
	return rules.PlanComposeNames(rules.PlanComposeNamesParams{
		Scans: detection.ComposeScans,
		Files: selected.Values(),
		Patch: true,
	}).Patches
}

// ── Extraction ──────────────────────────────────────────────────────────────

// extractProjectAnswers reads the answers from the final wizard model using the
// step-key→index map, tolerating absent steps (targeted re-init builds a subset).
func extractProjectAnswers(final components.WizardModel, detection domain.InitDetectionResult, idx map[string]int) domain.InitProjectAnswers {
	steps := final.Steps()
	answers := domain.InitProjectAnswers{}

	at := func(key string) int {
		if i, ok := idx[key]; ok {
			return i
		}
		return -1
	}

	if i := at(stepBasePath); i >= 0 {
		if m, ok := steps[i].Model.(components.TextInputModel); ok {
			answers.BasePath = m.Value()
		}
	}
	if answers.BasePath == "" {
		answers.BasePath = domain.DefaultBasePath
	}

	if i := at(stepBaseBranch); i >= 0 {
		if m, ok := steps[i].Model.(components.SelectListModel); ok {
			answers.BaseBranch = m.Value()
		}
	}
	if answers.BaseBranch == "" {
		answers.BaseBranch = detection.BaseBranch
	}

	// Env section.
	if i := at(stepEnvStrategy); i >= 0 {
		if final.Skipped(i) {
			answers.SkipEnv = true
		} else if m, ok := steps[i].Model.(components.SelectListModel); ok {
			answers.EnvStrategy = domain.EnvStrategy(m.Value())
			if fi := at(stepEnvFiles); fi >= 0 && !final.Skipped(fi) {
				if fm, ok := steps[fi].Model.(components.MultiSelectModel); ok {
					answers.EnvFiles = selectEnvFiles(detection.EnvFiles, fm.Values())
				}
			}
		}
	}

	// Hooks section.
	if i := at(stepHooks); i >= 0 {
		if final.Skipped(i) {
			answers.SkipHooks = true
		} else if m, ok := steps[i].Model.(components.HookListModel); ok {
			answers.OnCreate = m.Hooks()
		}
	}
	if i := at(stepHooksClean); i >= 0 {
		if final.Skipped(i) {
			answers.SkipClean = true
		} else if m, ok := steps[i].Model.(components.HookListModel); ok {
			answers.OnClean = m.Hooks()
		}
	}

	// Services section.
	if i := at(stepDocker); i >= 0 && !final.Skipped(i) {
		if m, ok := steps[i].Model.(components.MultiSelectModel); ok {
			answers.SelectionAsked = true
			answers.DockerComposeFiles = m.Values()
			if len(answers.DockerComposeFiles) > 0 {
				answers.DockerComposeCmd = detection.DockerComposeCmd
			}
		}
	}
	if i := at(stepComposePatch); i >= 0 && !final.Skipped(i) {
		if m, ok := steps[i].Model.(components.ConfirmModel); ok {
			answers.PatchCompose = m.Confirmed()
		}
	}
	if i := at(stepScripts); i >= 0 && !final.Skipped(i) {
		if m, ok := steps[i].Model.(components.MultiSelectModel); ok {
			answers.SelectionAsked = true
			for _, idxStr := range m.Values() {
				n, err := strconv.Atoi(idxStr)
				if err != nil || n < 0 || n >= len(detection.PackageScripts) {
					continue
				}
				answers.SelectedPackageScripts = append(answers.SelectedPackageScripts, detection.PackageScripts[n])
			}
		}
	}
	if i := at(stepPorts); i >= 0 && !final.Skipped(i) {
		if m, ok := steps[i].Model.(components.PortListModel); ok {
			answers.Ports = m.Entries()
		}
	}
	if i := at(stepProfiles); i >= 0 && !final.Skipped(i) {
		if m, ok := steps[i].Model.(components.ProfileListModel); ok {
			answers.Profiles = m.Profiles()
			answers.ProfilesAsked = true
		}
	}
	if i := at(stepRuns); i >= 0 && !final.Skipped(i) {
		if m, ok := steps[i].Model.(components.RunnerListModel); ok {
			answers.Runners = m.Choices()
		}
	}
	if i := at(stepAddressing); i >= 0 && !final.Skipped(i) {
		if m, ok := steps[i].Model.(components.SelectListModel); ok {
			answers.AddressingAsked, answers.Addressing = true, domain.Addressing(m.Value())
		}
	}
	if i := at(stepPortRoute); i >= 0 && !final.Skipped(i) {
		if m, ok := steps[i].Model.(components.RouteListModel); ok {
			answers.PortRoutesAsked, answers.PortRoutes = true, m.Routes()
		}
	}
	if i := at(stepCmds); i >= 0 && !final.Skipped(i) {
		if m, ok := steps[i].Model.(components.CmdListModel); ok {
			answers.Cmds = m.Fixes()
		}
	}
	if i := at(stepURLs); i >= 0 && !final.Skipped(i) {
		if m, ok := steps[i].Model.(components.MultiSelectModel); ok {
			answers.URLsAsked, answers.URLs = true, m.Values()
		}
	}
	if i := at(stepEnvLink); i >= 0 && !final.Skipped(i) {
		if m, ok := steps[i].Model.(components.ConfirmModel); ok {
			answers.EnvLinksAsked, answers.LinkEnv = true, m.Confirmed()
		}
	}
	if i := at(stepScriptKinds); i >= 0 && !final.Skipped(i) {
		if m, ok := steps[i].Model.(components.KindListModel); ok {
			answers.SelectedPackageScripts = rules.ApplyScriptKinds(rules.ApplyScriptKindsParams{
				Scripts: answers.SelectedPackageScripts,
				Choices: m.Entries(),
			})
		}
	}

	return answers
}

// ── Summaries & gate helpers ────────────────────────────────────────────────

func textInputSummary(model any) string {
	ti, ok := model.(components.TextInputModel)
	if !ok {
		return ""
	}
	v := ti.Value()
	if v != "" {
		return v
	}
	if p := ti.Placeholder(); p != "" {
		return p + " (default)"
	}
	return "(empty)"
}

func selectListSummary(model any) string {
	sl, ok := model.(components.SelectListModel)
	if !ok {
		return ""
	}
	return sl.Value()
}

func multiSelectSummary(model any) string {
	ms, ok := model.(components.MultiSelectModel)
	if !ok {
		return ""
	}
	return fmt.Sprintf("%d files selected", len(ms.Values()))
}

func urlListSummary(model any) string {
	ms, ok := model.(components.MultiSelectModel)
	if !ok {
		return ""
	}
	return fmt.Sprintf(domain.URLListSummaryFmt, len(ms.Values()))
}

func packageScriptsSummary(model any) string {
	ms, ok := model.(components.MultiSelectModel)
	if !ok {
		return ""
	}
	return fmt.Sprintf("%d scripts selected", len(ms.Values()))
}

func hookListSummary(model any) string {
	hl, ok := model.(components.HookListModel)
	if !ok {
		return ""
	}
	return fmt.Sprintf("%d command(s)", len(hl.Hooks()))
}

// sectionGateParams holds the inputs for a section gate step.
type sectionGateParams struct {
	Name           string
	Description    string
	Detected       string
	ConfigureLabel string
	SkipLabel      string
}

// sectionGate builds the intro/gate step for an optional section. Configure is
// always listed first (and thus highlighted by default) so the Configure/Skip
// order stays identical across every gate. The explanation is rendered as a
// callout above the choice.
func sectionGate(p sectionGateParams) components.Step {
	configure := components.SelectItem{Label: p.ConfigureLabel, Value: domain.WizardChoiceConfigure}
	skip := components.SelectItem{Label: p.SkipLabel, Value: domain.WizardChoiceSkip}

	items := []components.SelectItem{configure, skip}

	note := ""
	if p.Detected != "" {
		note = "Detected: " + p.Detected
	}

	return components.Step{
		Name: p.Name,
		Model: components.NewSelectList(components.NewSelectListParams{
			Title:       p.Name,
			Description: p.Description,
			Items:       items,
		}),
		Summary:     gateSummary,
		Callout:     true,
		CalloutNote: note,
	}
}

// gateSummary renders a section gate's chosen state in the breadcrumb.
func gateSummary(model any) string {
	sl, ok := model.(components.SelectListModel)
	if !ok {
		return ""
	}
	if sl.Value() == domain.WizardChoiceSkip {
		return "skipped"
	}
	return "configured"
}

// autoSkipWhenGateSkipped returns an AutoSkip predicate that skips a sub-step
// when its section gate (at gateIdx) is set to "skip".
func autoSkipWhenGateSkipped(gateIdx int) func(components.WizardModel) bool {
	return func(w components.WizardModel) bool {
		return gateValue(w, gateIdx) == domain.WizardChoiceSkip
	}
}

// gateValue reads the selected value of the gate SelectList at idx.
func gateValue(w components.WizardModel, idx int) string {
	steps := w.Steps()
	if idx < 0 || idx >= len(steps) {
		return ""
	}
	sl, ok := steps[idx].Model.(components.SelectListModel)
	if !ok {
		return ""
	}
	return sl.Value()
}

// prefillSelected decides a multiselect item's checked state. Without a prefill
// (full init) it uses the detection default; with a prefill it checks the items
// already present in the current config.
func prefillSelected(prefill *SectionPrefill, configured, fullInitDefault bool) bool {
	if prefill == nil {
		return fullInitDefault
	}
	return configured
}

// moveToFront returns items with the entry matching value placed first, so a
// SelectList highlights the currently-configured choice by default.
func moveToFront(items []components.SelectItem, value string) []components.SelectItem {
	if value == "" {
		return items
	}
	for i, item := range items {
		if item.Value == value {
			reordered := []components.SelectItem{item}
			reordered = append(reordered, items[:i]...)
			reordered = append(reordered, items[i+1:]...)
			return reordered
		}
	}
	return items
}

func detectedEnv(d domain.InitDetectionResult) string {
	labels := make([]string, 0, len(d.EnvFiles))
	for _, f := range d.EnvFiles {
		labels = append(labels, envFileLabel(f))
	}
	return strings.Join(labels, ", ")
}

// envFileLabel renders a detected env file for display: the value target, its
// committed template if any, and a local badge for machine-local overrides.
func envFileLabel(f domain.EnvFile) string {
	label := f.Target
	if f.Template != "" {
		label += " ← " + f.Template
	}
	if f.Local {
		label += " (local)"
	}
	return label
}

// selectEnvFiles keeps the detected files whose target was selected in the wizard,
// preserving the detection order and the template relation.
func selectEnvFiles(detected []domain.EnvFile, targets []string) []domain.EnvFile {
	chosen := make(map[string]bool, len(targets))
	for _, t := range targets {
		chosen[t] = true
	}
	files := make([]domain.EnvFile, 0, len(targets))
	for _, f := range detected {
		if chosen[f.Target] {
			files = append(files, f)
		}
	}
	return files
}

func detectedHooks(d domain.InitDetectionResult) string {
	return d.InstallCommand
}

// addComposePatchStep asks whether the selected compose files may be rewritten
// so their literal host ports read a variable. It only appears when the files
// picked in the previous step actually have something to rewrite, and it lists
// every line it would touch — the same lines the recap reports afterwards.
type addComposePatchStepParams struct {
	Detection domain.InitDetectionResult
	// Existing is the config on disk: the same collision and conflict checks the
	// recap runs must run here, or the step asks to rewrite lines it will not.
	Existing domain.RunConfig
	// Authorized is --patch-compose: the answer is already in, so the step
	// states it rather than asking, and stays in the recap.
	Authorized bool
	EnvScans   map[string]domain.EnvPortScan
}

func addComposePatchStep(s *stepSet, params addComposePatchStepParams) {
	detection := params.Detection
	docker, scripts := s.at(stepDocker), s.at(stepScripts)
	if docker < 0 || len(detection.ComposeScans) == 0 {
		return
	}

	s.add(stepComposePatch, components.ConfirmStep(components.ConfirmStepParams{
		Name:     domain.ComposePatchStepName,
		YesLabel: domain.ComposePatchStepYes,
		NoLabel:  domain.ComposePatchStepNo,
		Callout:  true,
		Decide: func(prev []components.Step) (bool, string, components.NewConfirmParams) {
			patches := composePatchesFor(composePatchesForParams{
				Prev:      prev,
				Docker:    docker,
				Scripts:   scripts,
				Detection: detection,
				Existing:  params.Existing,
				EnvScans:  params.EnvScans,
			})
			names := composeNamesFor(prev, docker, detection)
			if len(patches) == 0 && len(names) == 0 {
				return false, "", components.NewConfirmParams{}
			}
			if params.Authorized {
				return false, "--" + domain.FlagPatchCompose, components.NewConfirmParams{}
			}
			return true, "", components.NewConfirmParams{
				Title:       domain.ComposePatchStepTitle,
				Description: composePatchDescription(patches, names),
			}
		},
	}))
}

type composePatchesForParams struct {
	Prev      []components.Step
	Docker    int
	Scripts   int
	Detection domain.InitDetectionResult
	Existing  domain.RunConfig
	EnvScans  map[string]domain.EnvPortScan
}

// composePatchesFor runs the full resolution, not just the plan, so the lines
// the step asks about are exactly the ones the recap will report as rewritten.
func composePatchesFor(params composePatchesForParams) map[string][]domain.ComposePortBinding {
	answers := answersFromSteps(answersFromStepsParams{
		Prev:      params.Prev,
		Docker:    params.Docker,
		Scripts:   params.Scripts,
		Detection: params.Detection,
	})
	if len(answers.DockerComposeFiles) == 0 {
		return nil
	}

	return rules.ResolveDetectedPorts(rules.ResolveDetectedPortsParams{
		Answers:        answers,
		PackageManager: params.Detection.PackageManager,
		Existing:       params.Existing,
		Plan: rules.PlanComposePorts(rules.PlanComposePortsParams{
			Scans: params.Detection.ComposeScans,
			Files: answers.DockerComposeFiles,
			Patch: true,
		}),
		EnvScansByDir: params.EnvScans,
	}).Patches
}

type answersFromStepsParams struct {
	Prev      []components.Step
	Docker    int
	Scripts   int
	Detection domain.InitDetectionResult
}

// answersFromSteps reads the wizard's live selections as an answers struct, so a
// step that must plan against them sees exactly what the recap will. Several
// steps need this and they must not each build it their own way.
func answersFromSteps(params answersFromStepsParams) domain.InitProjectAnswers {
	answers := domain.InitProjectAnswers{
		DockerComposeCmd:       params.Detection.DockerComposeCmd,
		PatchCompose:           true,
		SelectedPackageScripts: selectedScripts(params.Prev, params.Scripts, params.Detection.PackageScripts),
	}
	if params.Docker >= 0 && params.Docker < len(params.Prev) {
		if selected, ok := params.Prev[params.Docker].Model.(components.MultiSelectModel); ok {
			answers.DockerComposeFiles = selected.Values()
		}
	}
	return answers
}

// selectedScripts reads a scripts multi-select back into the scripts it names,
// tolerating a step that is absent from this wizard.
func selectedScripts(prev []components.Step, at int, detected []domain.PackageScript) []domain.PackageScript {
	if at < 0 || at >= len(prev) {
		return nil
	}
	model, ok := prev[at].Model.(components.MultiSelectModel)
	if !ok {
		return nil
	}

	var scripts []domain.PackageScript
	for _, value := range model.Values() {
		n, err := strconv.Atoi(value)
		if err != nil || n < 0 || n >= len(detected) {
			continue
		}
		scripts = append(scripts, detected[n])
	}
	return scripts
}
