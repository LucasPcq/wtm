package clean

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/runconfig"
	"github.com/LucasPcq/wtm/internal/service/worktree"
)

const (
	KeyWorktree = "clean.worktree"
	KeyReparent = "clean.reparent"
	KeyDelete   = "clean.delete"
)

const (
	reparentYes = "reparent"
	reparentNo  = "orphan"
	deleteYes   = "yes"
	deleteForce = "force"
)

const (
	labelWorktree = "Worktree"
	labelReparent = "Reparent children"
	labelDelete   = "Delete"
)

// The order makes the delete confirmation the last, unconditional action point.
func (f *cleanFlow) session() flow.Session {
	return flow.Session{
		ErrLabel: domain.CleanWizardErrLabel,
		Presets: flow.NewAnswers(map[string]string{
			KeyWorktree: f.request.Branch,
			KeyReparent: f.presetReparent(),
		}),
		Steps: []flow.Step{
			f.worktreeStep(),
			f.reparentStep(),
			f.deleteStep(),
		},
	}
}

func (f *cleanFlow) worktreeStep() flow.Step {
	return flow.Step{
		Kind:        flow.StepSelect,
		Key:         KeyWorktree,
		Label:       labelWorktree,
		Title:       domain.CleanPickerTitle,
		Description: domain.CleanPickerDescription,
		Build: func(flow.Answers) (flow.StepContent, error) {
			options, err := f.cleanableOptions()
			if err != nil {
				return flow.StepContent{}, err
			}
			return flow.StepContent{
				Title:       domain.CleanPickerTitle,
				Description: domain.CleanPickerDescription,
				Options:     options,
			}, nil
		},
		Resolve: func(flow.Answers) (flow.Answer, error) {
			return flow.Answer{}, domain.ErrCleanBranchRequired
		},
	}
}

func (f *cleanFlow) cleanableOptions() ([]flow.Option, error) {
	worktrees, err := worktree.ListAll(worktree.ListAllParams{ProjectDir: f.ctx.ProjectDir})
	if err != nil {
		return nil, fmt.Errorf("list worktrees: %w", err)
	}
	options := make([]flow.Option, 0, len(worktrees))
	for _, wt := range worktrees {
		if wt.IsMain {
			continue
		}
		options = append(options, flow.Option{Label: wt.Branch, Value: wt.Branch})
	}
	sort.Slice(options, func(i, j int) bool { return options[i].Value < options[j].Value })
	if len(options) == 0 {
		return nil, errors.New(domain.CleanNothingToClean)
	}
	return options, nil
}

func (f *cleanFlow) reparentStep() flow.Step {
	return flow.Step{
		Kind:  flow.StepSelect,
		Key:   KeyReparent,
		Label: labelReparent,
		Skip: func(answers flow.Answers) (bool, string) {
			if len(f.reparentPlan(answers.Value(KeyWorktree)).Children) == 0 {
				return true, domain.CleanNoOrphanedChildren
			}
			return false, ""
		},
		Build: func(answers flow.Answers) (flow.StepContent, error) {
			plan := f.reparentPlan(answers.Value(KeyWorktree))
			return flow.StepContent{
				Description: reparentProposal(plan),
				Options: []flow.Option{
					{Label: fmt.Sprintf(domain.CleanReparentOptionFmt, plan.Grandparent, len(plan.Children)), Value: reparentYes},
					{Separator: true},
					{Label: domain.CleanOrphanOption, Value: reparentNo},
				},
			}, nil
		},
		// Reparenting is opt-in: `wtm reparent` can still move the children afterwards.
		Resolve: func(flow.Answers) (flow.Answer, error) {
			return flow.Answer{Value: reparentNo}, nil
		},
		Summarize: func(answer flow.Answer) string {
			if answer.Value == reparentYes {
				return domain.CleanReparentSummary
			}
			return domain.CleanOrphanSummary
		},
		Flag: domain.FlagReparentChildren,
	}
}

// presetReparent answers the step from --reparent-children, so the decision is not
// asked and every reader sees the same answer.
func (f *cleanFlow) presetReparent() string {
	if f.request.ReparentChildren {
		return reparentYes
	}
	return ""
}

func reparentProposal(plan domain.CleanReparentPlan) string {
	lines := make([]string, 0, len(plan.Children)+1)
	lines = append(lines, domain.CleanReparentIntro)
	for _, child := range plan.Children {
		lines = append(lines, fmt.Sprintf(domain.CleanReparentChildFmt, child.Branch, child.NewParent, child.OldParent))
	}
	return strings.Join(lines, "\n")
}

func (f *cleanFlow) deleteStep() flow.Step {
	content := func(answers flow.Answers) (flow.StepContent, error) {
		check, _ := f.checkCached(answers.Value(KeyWorktree))
		return flow.StepContent{
			Title: domain.CleanDeleteTitle,
			Description: deleteRecap(deleteRecapParams{
				Check:    check,
				Reparent: f.reparentLine(answers),
				Tenants:  f.tenantLines(answers.Value(KeyWorktree)),
			}),
			Options:  deleteOptions(check),
			Blockers: blockersOf(check),
		}, nil
	}

	step := flow.Step{
		Kind:           flow.StepRecap,
		Key:            KeyDelete,
		Label:          labelDelete,
		Title:          domain.CleanDeleteTitle,
		LoadingMessage: domain.CleanCheckLoading,
		Resolve:        f.resolveDelete,
	}
	// A branch given up front was already checked, so its recap needs no I/O. A
	// picked one is checked then and there, over the network.
	if f.request.Branch != "" {
		step.Build = content
		return step
	}
	step.Load = content
	return step
}

// resolveDelete keeps the safety check while answering: --yes is the confirmation
// axis, not a licence to remove something unsafe. Only --force lifts the refusal.
func (f *cleanFlow) resolveDelete(answers flow.Answers) (flow.Answer, error) {
	if f.request.Force {
		return flow.Answer{Value: deleteYes}, nil
	}
	check, err := f.checkStaged(answers.Value(KeyWorktree))
	if err != nil {
		// Absent, parent or unreadable: the removal reports all three idempotently.
		return flow.Answer{Value: deleteYes}, nil
	}
	if reason, unsafe := rules.CleanUnsafeReason(check); unsafe {
		return flow.Answer{}, fmt.Errorf(domain.CleanForceHintFmt, check.Branch, reason)
	}
	return flow.Answer{Value: deleteYes}, nil
}

func deleteOptions(check domain.CleanCheckResult) []flow.Option {
	options := []flow.Option{{Label: domain.CleanDeleteOption, Value: deleteYes}}
	if rules.HasWarnings(check) {
		options = append(options,
			flow.Option{Separator: true},
			flow.Option{Label: domain.CleanForceDeleteOption, Value: deleteForce, Danger: true},
		)
	}
	return options
}

// blockersOf carries the refusals to the surface, which decides whether to print
// them or to ask for each one to be lifted.
func blockersOf(check domain.CleanCheckResult) []flow.Blocker {
	refusals := rules.CleanBlockers(check)
	blockers := make([]flow.Blocker, 0, len(refusals))
	for _, refusal := range refusals {
		blockers = append(blockers, flow.Blocker{Key: refusal.Key, Label: refusal.Label})
	}
	return blockers
}

type deleteRecapParams struct {
	Check    domain.CleanCheckResult
	Reparent string
	// Tenants are the lines naming the data this clean gives back, or the one
	// saying it is kept. A flag must never make a line disappear from a recap,
	// and this one carries a DROP DATABASE.
	Tenants []string
}

func deleteRecap(params deleteRecapParams) string {
	check := params.Check
	var lines []string
	for _, blocker := range rules.CleanBlockers(check) {
		lines = append(lines, blocker.Label)
	}
	if len(lines) > 0 {
		lines = append(lines, "")
	}
	lines = append(lines,
		domain.CleanWillDelete,
		domain.CleanWillDeleteWorktree+check.WorktreePath,
		domain.CleanWillDeleteBranch+check.Branch,
	)
	lines = append(lines, params.Tenants...)
	if params.Reparent != "" {
		lines = append(lines, "", params.Reparent)
	}
	return strings.Join(lines, "\n")
}

// tenantLines says what this clean does to the data the worktree carved out of
// the repository's shared services — given back by default, kept under
// --keep-data. Silence is not an option: the default runs a DROP DATABASE.
func (f *cleanFlow) tenantLines(branchName string) []string {
	cfg, err := runconfig.Load(f.ctx.StateDir)
	if err != nil {
		return nil
	}
	held := worktree.TenantsOf(worktree.ParentBranchParams{StateDir: f.ctx.StateDir, Branch: branchName})
	if len(held) == 0 {
		return nil
	}
	slug := rules.WorktreeSlug(branchName)

	var lines []string
	for _, job := range rules.JobsHeld(cfg, held).Jobs {
		if !rules.HasTenant(job) || rules.IsBlankCommand(job.Tenant.Detach) {
			continue
		}
		if f.request.KeepData {
			return []string{domain.CleanKeepDataLine}
		}
		tenant, expandErr := rules.ExpandTenant(rules.ExpandTenantParams{Tenant: *job.Tenant, Worktree: slug})
		if expandErr != nil {
			continue
		}
		lines = append(lines, fmt.Sprintf(domain.CleanWillDeleteTenantFmt, tenant.Name, job.Name))
	}
	return lines
}

func (f *cleanFlow) reparentLine(answers flow.Answers) string {
	branchName := answers.Value(KeyWorktree)
	if branchName == "" {
		return ""
	}
	plan := f.reparentPlan(branchName)
	if len(plan.Children) == 0 {
		return ""
	}
	if answers.Value(KeyReparent) == reparentYes {
		return fmt.Sprintf(domain.CleanRecapReparentFmt, len(plan.Children), plan.Grandparent)
	}
	return fmt.Sprintf(domain.CleanRecapOrphanFmt, len(plan.Children))
}
