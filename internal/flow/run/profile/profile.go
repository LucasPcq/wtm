// Package profile runs the `wtm run profile add|edit|rm|list` flows — CRUD on
// the profile declarations in run.toml, on the same model as the job ones.
package profile

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/flow/run/target"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/runconfig"
)

type Presenter interface {
	flow.Presenter
	Changed(Outcome) error
}

type Outcome struct {
	Name    string
	Status  string
	Aborted bool
}

func aborted(presenter Presenter) (Outcome, error) {
	presenter.Notice(flow.AbortedNotice)
	return Outcome{Aborted: true}, nil
}

type AddRequest struct {
	Initial domain.ProfileConfig
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

	added := fromAnswers(answers)
	previous := rules.FindExistingDefaultProfile(params.Request.Config, "")
	cfg := params.Request.Config
	cfg.Profiles = append(cfg.Profiles, added)
	if added.Default {
		cfg = rules.ApplyDefaultOverride(cfg, added.Name)
	}
	if err := save(ctx, params.Context, cfg); err != nil {
		return Outcome{}, err
	}
	sayDefaultReplaced(defaultReplacedParams{Presenter: params.Presenter, Previous: previous, Current: defaultName(added)})
	return conclude(params.Presenter, Outcome{Name: added.Name, Status: domain.JobActionAdded})
}

type EditRequest struct {
	Name   string
	Patch  rules.ProfilePatch
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
		Step:     pickStep(params.Request.Config, domain.RunProfilePickerTitleEdit),
		Given:    params.Request.Name,
		Subject:  domain.CmdProfile,
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
	current, exists := rules.FindProfile(params.Request.Config, name)
	if !exists {
		return Outcome{}, fmt.Errorf(domain.RunProfileNotFoundFmt, domain.ErrProfileNotFound, name)
	}

	updated, err := editedProfile(params, current)
	if errors.Is(err, domain.ErrUserAborted) {
		return aborted(params.Presenter)
	}
	if err != nil {
		return Outcome{}, err
	}

	previous := rules.FindExistingDefaultProfile(params.Request.Config, "")
	if previous == current.Name {
		previous = ""
	}
	cfg := params.Request.Config
	for i, profile := range cfg.Profiles {
		if profile.Name == current.Name {
			cfg.Profiles[i] = updated
			break
		}
	}
	if updated.Default {
		cfg = rules.ApplyDefaultOverride(cfg, updated.Name)
	}
	if err := save(ctx, params.Context, cfg); err != nil {
		return Outcome{}, err
	}
	sayDefaultReplaced(defaultReplacedParams{Presenter: params.Presenter, Previous: previous, Current: defaultName(updated)})
	return conclude(params.Presenter, Outcome{Name: updated.Name, Status: domain.JobActionUpdated})
}

func editedProfile(params EditParams, current domain.ProfileConfig) (domain.ProfileConfig, error) {
	if !params.Request.Patch.Empty() {
		return rules.ApplyProfilePatch(rules.ApplyProfilePatchParams{Current: current, Patch: params.Request.Patch})
	}
	if !params.Prompter.Interactive() {
		return domain.ProfileConfig{}, fmt.Errorf(domain.RunProfileNothingToEdit,
			domain.FlagName, domain.FlagJobs, domain.FlagDefault)
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
		return domain.ProfileConfig{}, err
	}
	return fromAnswers(answers), nil
}

type RemoveRequest struct {
	Name   string
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
		Step:     pickStep(params.Request.Config, domain.RunProfilePickerTitleRemove),
		Given:    params.Request.Name,
		Subject:  domain.CmdProfile,
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

// removeNamed leaves the jobs the profile started untouched: a profile is a way
// of naming them together, not what they belong to.
func removeNamed(ctx context.Context, params RemoveParams, name string) (Outcome, error) {
	removed, exists := rules.FindProfile(params.Request.Config, name)
	if !exists {
		return Outcome{}, fmt.Errorf(domain.RunProfileNotFoundFmt, domain.ErrProfileNotFound, name)
	}

	cfg := params.Request.Config
	cfg.Profiles = slices.DeleteFunc(cfg.Profiles, func(p domain.ProfileConfig) bool { return p.Name == name })
	if err := save(ctx, params.Context, cfg); err != nil {
		return Outcome{}, err
	}
	sayDefaultRemoved(defaultRemovedParams{Presenter: params.Presenter, Removed: removed, Config: cfg})
	return conclude(params.Presenter, Outcome{Name: name, Status: domain.JobActionRemoved})
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

func List(ctx context.Context, params ListParams) (Outcome, error) {
	answers, err := params.Prompter.Ask(flow.Session{
		ErrLabel: domain.CmdList,
		Steps: []flow.Step{
			pickStep(params.Request.Config, domain.RunProfilePickerTitleChoose),
			actionStep(),
		},
	})
	if errors.Is(err, domain.ErrUserAborted) {
		return aborted(params.Presenter)
	}
	if err != nil {
		return Outcome{}, err
	}

	name := answers.Value(target.KeyProfile)
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

func defaultName(profile domain.ProfileConfig) string {
	if !profile.Default {
		return ""
	}
	return profile.Name
}

type defaultReplacedParams struct {
	Presenter Presenter
	Previous  string
	Current   string
}

func sayDefaultReplaced(params defaultReplacedParams) {
	if params.Previous == "" || params.Current == "" || params.Previous == params.Current {
		return
	}
	params.Presenter.Notice(flow.Notice{
		Kind: flow.NoticeWarning,
		Text: fmt.Sprintf(domain.RunProfileDefaultReplacedFmt, params.Previous, params.Current),
	})
}

type defaultRemovedParams struct {
	Presenter Presenter
	Removed   domain.ProfileConfig
	Config    domain.RunConfig
}

func sayDefaultRemoved(params defaultRemovedParams) {
	if !params.Removed.Default || len(params.Config.Profiles) == 0 {
		return
	}
	text := fmt.Sprintf(domain.RunProfileNoDefaultLeftFmt, params.Removed.Name)
	if fallback, ok := rules.DefaultProfile(params.Config); ok {
		text = fmt.Sprintf(domain.RunProfileDefaultRemovedFmt, params.Removed.Name, fallback.Name)
	}
	params.Presenter.Notice(flow.Notice{Kind: flow.NoticeWarning, Text: text})
}

func pickStep(cfg domain.RunConfig, title string) flow.Step {
	return target.ProfilePickStep(target.ProfilePickParams{Profiles: cfg.Profiles, Title: title})
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
