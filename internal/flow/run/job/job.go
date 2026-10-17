// Package job runs the `wtm run job add|edit|rm|list` flows — CRUD on the job
// declarations in run.toml. They mutate a config rather than a worktree, but
// they ask questions and two surfaces have to be able to ask them, which is the
// whole reason flow/ exists.
package job

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/flow/run/target"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/runconfig"
)

type Presenter interface {
	flow.Presenter
	// Changed concludes the run with what the file now says. It is typed rather
	// than a formatted line so the CLI can print it and a machine surface can
	// serialise it.
	Changed(Outcome) error
}

type Outcome struct {
	Name   string
	Status string
	// Effect is what a removal dragged along with the job — the profiles that
	// named it, the ones it emptied, the .env links that followed its ports.
	Effect  rules.RemoveJobEffect
	Aborted bool
}

// aborted is the one conclusion that is not a change: the user backing out is
// not a failure, so it is a notice and a nil error.
func aborted(presenter Presenter) (Outcome, error) {
	presenter.Notice(flow.AbortedNotice)
	return Outcome{Aborted: true}, nil
}

type AddRequest struct {
	// Initial is what the positional and the flags already filled in. It is the
	// form's pre-fill and, field by field, the answer of a run that cannot be
	// asked — a name and a command with neither are what a refusal names.
	Initial domain.JobConfig
	Config  domain.RunConfig
}

type AddParams struct {
	Context   flow.Context
	Request   AddRequest
	Prompter  flow.Prompter
	Presenter Presenter
}

func Add(ctx context.Context, params AddParams) (Outcome, error) {
	answers, err := params.Prompter.Ask(flow.Session{
		ErrLabel: domain.CmdAdd,
		Steps: formSteps(formParams{
			Existing: params.Request.Config,
			Initial:  params.Request.Initial,
		}),
	})
	if errors.Is(err, domain.ErrUserAborted) {
		return aborted(params.Presenter)
	}
	if err != nil {
		return Outcome{}, err
	}

	job, err := fromAnswers(answers, params.Request.Initial)
	if err != nil {
		return Outcome{}, err
	}

	cfg := params.Request.Config
	cfg.Jobs = append(cfg.Jobs, job)
	if err := save(ctx, params.Context, cfg); err != nil {
		return Outcome{}, err
	}
	return conclude(params.Presenter, Outcome{Name: job.Name, Status: domain.JobActionAdded})
}

type EditRequest struct {
	// Name is the positional; empty opens the picker. Patch is the
	// non-interactive edit: a non-empty one decides on its own, which is what
	// keeps `run job edit api --cmd x` from re-asking the six other fields.
	Name   string
	Patch  rules.JobPatch
	Config domain.RunConfig
}

type EditParams struct {
	Context   flow.Context
	Request   EditRequest
	Prompter  flow.Prompter
	Presenter Presenter
}

func Edit(ctx context.Context, params EditParams) (Outcome, error) {
	name, err := target.PickOne(target.PickOneParams{
		Prompter: params.Prompter,
		Step:     pickStep(params.Request.Config, domain.RunJobPickerTitleEdit),
		Given:    params.Request.Name,
		Subject:  domain.CmdJob,
		Label:    domain.CmdEdit,
	})
	if errors.Is(err, domain.ErrUserAborted) {
		return aborted(params.Presenter)
	}
	if err != nil {
		return Outcome{}, err
	}
	return editNamed(ctx, params, name)
}

func editNamed(ctx context.Context, params EditParams, name string) (Outcome, error) {
	current, exists := rules.FindJob(params.Request.Config, name)
	if !exists {
		return Outcome{}, fmt.Errorf(domain.RunJobNotFoundFmt, domain.ErrJobNotFound, name)
	}

	updated, err := editedJob(params, current)
	if errors.Is(err, domain.ErrUserAborted) {
		return aborted(params.Presenter)
	}
	if err != nil {
		return Outcome{}, err
	}

	if rules.SameJob(updated, current) {
		return conclude(params.Presenter, Outcome{Name: current.Name, Status: domain.JobActionUnchanged})
	}
	renamed := updated.Name != current.Name
	if held := heldBy(params.Context, current.Name); renamed && len(held) > 0 {
		return Outcome{}, fmt.Errorf(domain.RunJobRenameHeldFmt, current.Name, strings.Join(held, domain.RunURLListSep))
	}

	cfg := rules.RenameJobRefs(rules.RenameJobRefsParams{Config: params.Request.Config, From: current.Name, To: updated.Name})
	for i, job := range cfg.Jobs {
		if job.Name == current.Name {
			cfg.Jobs[i] = updated
			break
		}
	}
	if err := save(ctx, params.Context, cfg); err != nil {
		return Outcome{}, err
	}
	if renamed && publishedUnderName(current) && publishedUnderName(updated) {
		params.Presenter.Notice(flow.Notice{
			Kind: flow.NoticeWarning,
			Text: fmt.Sprintf(domain.RunJobHostMovedFmt, current.Name, updated.Name, current.Name),
		})
	}
	return conclude(params.Presenter, Outcome{Name: updated.Name, Status: domain.JobActionUpdated})
}

func editedJob(params EditParams, current domain.JobConfig) (domain.JobConfig, error) {
	if !params.Request.Patch.Empty() {
		return rules.ApplyJobPatch(rules.ApplyJobPatchParams{Current: current, Patch: params.Request.Patch})
	}
	// The form is the only other way to change something, so a run that cannot
	// open it is refused naming what it could have passed instead. Every step
	// would otherwise resolve to the value it was pre-filled with, and the edit
	// would write the job back unchanged without a word.
	if !params.Prompter.Interactive() {
		return domain.JobConfig{}, fmt.Errorf(domain.RunJobNothingToEdit, editFlagList())
	}

	answers, err := params.Prompter.Ask(flow.Session{
		ErrLabel: domain.CmdEdit,
		Steps: formSteps(formParams{
			Existing:    params.Request.Config,
			Initial:     current,
			ExcludeName: current.Name,
		}),
	})
	if err != nil {
		return domain.JobConfig{}, err
	}
	return fromAnswers(answers, current)
}

var editFlags = []string{
	domain.FlagName, domain.FlagCmd, domain.FlagKind, domain.FlagStop, domain.FlagCwd,
	domain.FlagPort, domain.FlagPortClear, domain.FlagURLPort, domain.FlagURLHost,
	domain.FlagRuns, domain.FlagBindsNoPort, domain.FlagTouches, domain.FlagScope,
	domain.FlagNamespaceName, domain.FlagNamespaceCreate, domain.FlagNamespaceRemove, domain.FlagNamespaceEnv,
}

func editFlagList() string {
	named := make([]string, len(editFlags))
	for i, flag := range editFlags {
		named[i] = "--" + flag
	}
	return strings.Join(named, domain.RunURLListSep)
}

type RemoveRequest struct {
	Name string
	// Force is the safety axis: a job other declarations name is refused until
	// it says they may go with it. It is not the only way through — a run with
	// someone to ask lifts the same refusal by answering, which is what lets
	// `run job list` remove such a job without a flag it does not have.
	Force  bool
	Config domain.RunConfig
}

type RemoveParams struct {
	Context   flow.Context
	Request   RemoveRequest
	Prompter  flow.Prompter
	Presenter Presenter
}

func Remove(ctx context.Context, params RemoveParams) (Outcome, error) {
	name, err := target.PickOne(target.PickOneParams{
		Prompter: params.Prompter,
		Step:     pickStep(params.Request.Config, domain.RunJobPickerTitleRemove),
		Given:    params.Request.Name,
		Subject:  domain.CmdJob,
		Label:    domain.CmdRm,
	})
	if errors.Is(err, domain.ErrUserAborted) {
		return aborted(params.Presenter)
	}
	if err != nil {
		return Outcome{}, err
	}
	return removeNamed(ctx, params, name)
}

func removeNamed(ctx context.Context, params RemoveParams, name string) (Outcome, error) {
	if _, exists := rules.FindJob(params.Request.Config, name); !exists {
		return Outcome{}, fmt.Errorf(domain.RunJobNotFoundFmt, domain.ErrJobNotFound, name)
	}

	cfg, effect := rules.RemoveJob(params.Request.Config, name)
	held := heldBy(params.Context, name)
	if clauses := refusalClauses(effect, held); len(clauses) > 0 && !params.Request.Force {
		lifted, err := liftReference(params.Prompter, name, strings.Join(clauses, domain.RunJobClauseSep))
		if err != nil {
			return Outcome{}, err
		}
		if !lifted {
			return aborted(params.Presenter)
		}
	}
	if err := save(ctx, params.Context, cfg); err != nil {
		return Outcome{}, err
	}
	if len(held) > 0 {
		params.Presenter.Notice(flow.Notice{
			Kind: flow.NoticeWarning,
			Text: fmt.Sprintf(domain.RunJobHeldDroppedFmt, strings.Join(held, domain.RunURLListSep), name),
		})
	}
	return conclude(params.Presenter, Outcome{Name: name, Status: domain.JobActionRemoved, Effect: effect})
}

// refusalClauses is everything a removal would lose, which is what the refusal
// has to name: a reader deciding whether to lift it needs to know what goes.
func refusalClauses(effect rules.RemoveJobEffect, held []string) []string {
	var clauses []string
	if parts := rules.JobReferenceParts(effect); len(parts) > 0 {
		clauses = append(clauses, fmt.Sprintf(domain.RunJobRefClauseFmt, strings.Join(parts, domain.RunJobRefSep)))
	}
	if len(held) > 0 {
		clauses = append(clauses, fmt.Sprintf(domain.RunJobHeldClauseFmt, strings.Join(held, domain.RunURLListSep)))
	}
	return clauses
}

// liftReference asks to lift the safety refusal, and refuses naming --force
// when there is nobody to ask. Both routes converge on one value, the way
// clean's blockers do.
func liftReference(prompter flow.Prompter, name, reason string) (bool, error) {
	if !prompter.Interactive() {
		return false, fmt.Errorf(domain.RunJobReferencedFmt, name, reason, domain.FlagForce)
	}
	return prompter.Confirm(flow.ConfirmParams{
		Title:       domain.RunJobReferencedTitle,
		Description: fmt.Sprintf(domain.RunJobReferencedDescFmt, name, reason),
		YesLabel:    domain.RunJobReferencedYes,
		NoLabel:     domain.RunJobReferencedNo,
	})
}

type ListRequest struct {
	Config domain.RunConfig
}

type ListParams struct {
	Context   flow.Context
	Request   ListRequest
	Prompter  flow.Prompter
	Presenter Presenter
}

// List is the listing that acts: a job, then what to do to it. The chosen action
// runs here rather than in the surface, so the picker and the commands it stands
// for cannot drift apart.
func List(ctx context.Context, params ListParams) (Outcome, error) {
	answers, err := params.Prompter.Ask(flow.Session{
		ErrLabel: domain.CmdList,
		Steps: []flow.Step{
			pickStep(params.Request.Config, domain.RunJobPickerTitleChoose),
			actionStep(),
		},
	})
	if errors.Is(err, domain.ErrUserAborted) {
		return aborted(params.Presenter)
	}
	if err != nil {
		return Outcome{}, err
	}

	name := answers.Value(target.KeyJob)
	switch answers.Value(KeyAction) {
	case domain.RunCRUDActionEditValue:
		return editNamed(ctx, EditParams{
			Context:   params.Context,
			Request:   EditRequest{Name: name, Config: params.Request.Config},
			Prompter:  params.Prompter,
			Presenter: params.Presenter,
		}, name)
	case domain.RunCRUDActionRmValue:
		return removeNamed(ctx, RemoveParams{
			Context:   params.Context,
			Request:   RemoveRequest{Name: name, Config: params.Request.Config},
			Prompter:  params.Prompter,
			Presenter: params.Presenter,
		}, name)
	}
	return Outcome{Aborted: true}, nil
}

func pickStep(cfg domain.RunConfig, title string) flow.Step {
	return target.JobStep(target.JobParams{Jobs: cfg.Jobs, Title: title, Detail: true})
}

func save(ctx context.Context, project flow.Context, cfg domain.RunConfig) error {
	return runconfig.Save(ctx, runconfig.SaveParams{StateDir: project.StateDir, Config: cfg})
}

func conclude(presenter Presenter, outcome Outcome) (Outcome, error) {
	if err := presenter.Changed(outcome); err != nil {
		return Outcome{}, err
	}
	return outcome, nil
}
