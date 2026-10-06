package env

import (
	"fmt"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/rules"
)

const (
	KeyWorktree   = "env.worktree"
	KeyIsolation  = "env.isolation"
	KeyAddressing = "env.addressing"
	KeyResolve    = "env.resolve"
	KeyRecap      = "env.recap"
)

func (f *envFlow) session() flow.Session {
	return flow.Session{
		ErrLabel: domain.EnvWizardErrLabel,
		Presets: flow.NewAnswers(map[string]string{
			KeyWorktree:   f.request.Worktree,
			KeyIsolation:  string(f.request.Isolation),
			KeyAddressing: string(f.request.Addressing),
		}),
		Steps: []flow.Step{f.worktreeStep(), f.isolationStep(), f.addressingStep(), f.resolveStep(), f.recapStep()},
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
		badges = append(badges, driftBadge(f.scans[f.pickerKey(status.Branch)]))
		refused := f.refused[status.Branch]
		if refused != "" {
			badges = append(badges, flow.Badge{Text: fmt.Sprintf(domain.EnvBadgeRefusesFmt, refused), Tone: domain.ToneDanger})
		}
		options = append(options, flow.Option{Label: status.Branch, Value: status.Branch, Badges: badges, Disabled: refused != ""})
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

// driftBadge counts the port pass with the keys: a worktree whose only drift is
// a port to move is not in sync.
func driftBadge(scan branchScan) flow.Badge {
	count := rules.EnvDriftCount(scan.files) + len(rules.EnvPortRewrites(scan.ports)) + len(rules.OwnedEnvRewrites(scan.ports))
	if count == 0 {
		return flow.Badge{Text: domain.EnvBadgeInSync, Tone: domain.ToneSuccess}
	}
	return flow.Badge{Text: fmt.Sprintf(domain.EnvBadgeChangesFmt, count), Tone: domain.ToneWarning}
}

// isolationStep keeps or switches a linked worktree's isolation. Keeping opens
// under the cursor: reconciling the keys is what `wtm env` is usually run for,
// and a worktree created before the choice existed keeps its data where it is.
func (f *envFlow) isolationStep() flow.Step {
	return flow.Step{
		Kind:  flow.StepSelect,
		Key:   KeyIsolation,
		Label: domain.EnvIsolationStepLabel,
		Flag:  domain.FlagIsolation,
		Skip: func(answers flow.Answers) (bool, string) {
			state, err := f.answeredState(answers)
			switch {
			case err != nil || state.asksIsolation():
				return false, ""
			case state.isMain:
				return true, domain.EnvIsolationMainSkip
			}
			return true, domain.IsolationStepIrrelevant
		},
		Build: func(answers flow.Answers) (flow.StepContent, error) {
			state, err := f.answeredState(answers)
			if err != nil {
				return flow.StepContent{}, err
			}
			content := flow.StepContent{
				Title:       fmt.Sprintf(domain.EnvIsolationTitleFmt, answers.Value(KeyWorktree)),
				Description: domain.EnvIsolationDescription,
				Options:     isolationOptions(state),
			}
			if state.adoption.Pending {
				content.Description = domain.IsolationAdoptDescription
			}
			return content, nil
		},
		Resolve: keep,
		Summarize: func(answer flow.Answer) string {
			if answer.Value == domain.EnvKeepValue {
				return domain.IsolationAdoptKeptSummary
			}
			return rules.IsolationSummary(domain.Isolation(answer.Value))
		},
	}
}

func isolationOptions(state modeState) []flow.Option {
	switch {
	case state.adoption.Pending:
		return []flow.Option{
			{Label: domain.IsolationAdoptKeepLabel, Value: domain.EnvKeepValue},
			{Label: rules.IsolationAdoptOptionLabel(state.adoption), Value: string(domain.IsolationIsolated), Danger: state.adoption.ComposeProject != ""},
			{Label: domain.EnvIsolationAdoptVerbatim, Value: string(domain.IsolationVerbatim)},
		}
	case rules.IsVerbatim(state.recorded):
		return []flow.Option{
			{Label: domain.EnvIsolationKeepVerbatim, Value: domain.EnvKeepValue},
			{Label: domain.EnvIsolationToIsolated, Value: string(domain.IsolationIsolated)},
		}
	}
	return []flow.Option{
		{Label: domain.EnvIsolationKeepIsolated, Value: domain.EnvKeepValue},
		{Label: domain.EnvIsolationToVerbatim, Value: string(domain.IsolationVerbatim)},
	}
}

// addressingStep is the main checkout's: no pass over every worktree moves it
// onto names, so this is the one place it is asked, and keeping what its .env
// spells opens under the cursor.
func (f *envFlow) addressingStep() flow.Step {
	return flow.Step{
		Kind:  flow.StepSelect,
		Key:   KeyAddressing,
		Label: domain.EnvAddressingStepLabel,
		Flag:  domain.FlagAddressing,
		Skip: func(answers flow.Answers) (bool, string) {
			state, err := f.answeredState(answers)
			switch {
			case err != nil || state.asksAddressing():
				return false, ""
			case !state.isMain:
				return true, domain.EnvAddressingLinkedSkip
			}
			return true, domain.EnvAddressingSkip
		},
		Build: func(answers flow.Answers) (flow.StepContent, error) {
			state, err := f.answeredState(answers)
			if err != nil {
				return flow.StepContent{}, err
			}
			return flow.StepContent{
				Title:       fmt.Sprintf(domain.EnvAddressingTitleFmt, answers.Value(KeyWorktree)),
				Description: domain.EnvAddressingDescription,
				Options:     addressingOptions(state.current),
			}, nil
		},
		Resolve: keep,
		Summarize: func(answer flow.Answer) string {
			if answer.Value == domain.EnvKeepValue {
				return domain.IsolationAdoptKeptSummary
			}
			return answer.Value
		},
	}
}

func addressingOptions(current domain.Addressing) []flow.Option {
	if current == domain.AddressingPorts {
		return []flow.Option{
			{Label: domain.EnvAddressingKeepPorts, Value: domain.EnvKeepValue},
			{Label: domain.EnvAddressingToNames, Value: string(domain.AddressingNames)},
		}
	}
	return []flow.Option{
		{Label: domain.EnvAddressingKeepNames, Value: domain.EnvKeepValue},
		{Label: domain.EnvAddressingToPorts, Value: string(domain.AddressingPorts)},
	}
}

// keep is the unattended answer of both mode steps: a run with no flag never
// changes how a worktree runs.
func keep(flow.Answers) (flow.Answer, error) {
	return flow.Answer{Value: domain.EnvKeepValue}, nil
}

// resolveStep is skipped when no key is left to decide: an addition is one, since
// it can be skipped.
func (f *envFlow) resolveStep() flow.Step {
	return flow.Step{
		Kind:  flow.StepEnvResolve,
		Key:   KeyResolve,
		Label: domain.EnvResolveStepLabel,
		Skip: func(answers flow.Answers) (bool, string) {
			if !f.prompter.Interactive() {
				return true, domain.EnvResolveSkipReason
			}
			scan, err := f.scanOf(answers)
			if err != nil || rules.EnvResolvable(scan.files) {
				return false, ""
			}
			return true, domain.EnvResolveSkipReason
		},
		Build: func(answers flow.Answers) (flow.StepContent, error) {
			scan, err := f.scanOf(answers)
			if err != nil {
				return flow.StepContent{}, err
			}
			return flow.StepContent{
				Title:    fmt.Sprintf(domain.EnvResolveTitleFmt, answers.Value(KeyWorktree)),
				EnvFiles: scan.files,
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
		// Nobody is asked to confirm a run that would write nothing; an
		// unattended one is applied without reading its scan.
		Skip: func(answers flow.Answers) (bool, string) {
			if !f.prompter.Interactive() {
				return false, ""
			}
			applies, err := f.applies(answers)
			return err == nil && !applies, ""
		},
		Build: func(answers flow.Answers) (flow.StepContent, error) {
			description, err := f.recap(answers)
			if err != nil {
				return flow.StepContent{}, err
			}
			return flow.StepContent{
				Description: description,
				Options:     []flow.Option{{Label: domain.EnvApplyActionLabel, Value: domain.EnvApplyValue}},
			}, nil
		},
		Resolve: func(flow.Answers) (flow.Answer, error) {
			return flow.Answer{Value: domain.EnvApplyValue}, nil
		},
	}
}

// recap restates the worktree, every decision with its value, and the port
// values the apply will shift — announced before it happens rather than
// discovered after.
func (f *envFlow) recap(answers flow.Answers) (string, error) {
	branch := answers.Value(KeyWorktree)
	state, err := f.answeredState(answers)
	if err != nil {
		return "", err
	}
	scan, err := f.scanOf(answers)
	if err != nil {
		return "", err
	}
	isolation := f.isolation(answers)
	lines := []string{
		domain.EnvRecapFieldWorktree + branch,
		domain.RecapFieldMode + string(f.request.Mode),
		domain.RecapFieldEnv + f.sourceLabel(branch),
	}
	// Read from the state rather than the answers' Skipped: a step the wizard
	// skipped on its way here reaches the recap carrying its first option.
	if state.asksIsolation() || f.request.Isolation != "" {
		lines = append(lines, domain.RecapFieldIsolation+isolationRecap(isolationRecapParams{State: state, Isolation: isolation}))
	}
	if state.asksAddressing() || f.request.Addressing != "" {
		lines = append(lines, domain.EnvRecapFieldAddressing+addressingRecap(addressingRecapParams{State: state, Addressing: f.addressing(answers)}))
	}
	lines = append(lines, "")

	resolve, _ := answers.Get(KeyResolve)
	if body := rules.EnvResolveRecapLines(rules.EnvResolveRecapParams{Files: scan.files, Decisions: resolve.EnvDecisions}); len(body) > 0 && !resolve.Skipped {
		lines = append(lines, body...)
	} else {
		lines = append(lines, domain.EnvRecapSafeOnly)
	}
	lines = append(lines, rules.EnvPortRecapLines(scan.ports)...)
	// A worktree moving onto isolation was scanned without its port pass, which
	// would have allocated its ordinal before the answer was confirmed.
	if state.movesOntoIsolation(isolation) {
		lines = append(lines, "", domain.EnvRecapAdoptPorts)
	}
	if rules.IsVerbatim(isolation) {
		lines = append(lines, rules.EnvRestoreRecapLines(scan.restore)...)
	}
	return strings.Join(lines, "\n"), nil
}

type isolationRecapParams struct {
	State     modeState
	Isolation domain.Isolation
}

func isolationRecap(params isolationRecapParams) string {
	switch {
	case params.Isolation != "":
		return rules.IsolationSummary(params.Isolation)
	case params.State.adoption.Pending:
		return domain.IsolationAdoptKeptSummary
	}
	return rules.IsolationSummary(rules.EffectiveIsolation(params.State.recorded)) + domain.EnvRecapUnchanged
}

type addressingRecapParams struct {
	State      modeState
	Addressing domain.Addressing
}

func addressingRecap(params addressingRecapParams) string {
	if params.Addressing == "" || params.Addressing == params.State.current {
		return string(params.State.current) + domain.EnvRecapUnchanged
	}
	return string(params.Addressing)
}

// applies reports whether confirming would write anything: a key, a port, an
// owned value, or an isolation to record.
func (f *envFlow) applies(answers flow.Answers) (bool, error) {
	scan, err := f.scanOf(answers)
	if err != nil {
		return false, err
	}
	return rules.EnvDriftCount(scan.files) > 0 ||
		len(rules.EnvPortRewrites(scan.ports)) > 0 ||
		len(rules.OwnedEnvRewrites(scan.ports)) > 0 ||
		f.isolation(answers) != "", nil
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

func (f *envFlow) scanOf(answers flow.Answers) (branchScan, error) {
	return f.scanFor(scanKey{branch: answers.Value(KeyWorktree), isolation: f.isolation(answers), addressing: f.addressing(answers)})
}
