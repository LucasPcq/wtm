package profile

import (
	"errors"
	"fmt"
	"strings"
	"unicode"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/flow/run/target"
	"github.com/LucasPcq/wtm/internal/rules"
)

const (
	KeyName    = "run.profile.name"
	KeyJobs    = "run.profile.jobs"
	KeyOrder   = "run.profile.order"
	KeyDefault = "run.profile.default"
	KeyAction  = "run.profile.action"
)

type formParams struct {
	Existing domain.RunConfig
	// Initial is what the form opens on and, field by field, what a run that
	// cannot be asked answers with.
	Initial domain.ProfileConfig
	// ExcludeName is the name an edit may keep, a profile not being a duplicate
	// of itself.
	ExcludeName string
}

// formSteps is a profile declaration as a session: its name, what it starts, in
// what order, and whether it is the one `run up` takes by default.
func formSteps(params formParams) []flow.Step {
	return []flow.Step{
		{
			Kind:        flow.StepText,
			Key:         KeyName,
			Label:       domain.RunCRUDNameStepName,
			Title:       domain.RunProfileNameTitle,
			Description: domain.RunProfileNameDesc,
			Default:     params.Initial.Name,
			Arg:         true,
			Validate:    validateName(params),
			Resolve:     resolveGiven(params.Initial.Name),
		},
		{
			Kind:        flow.StepMultiSelect,
			Key:         KeyJobs,
			Label:       domain.RunProfileJobsLabel,
			Title:       domain.RunProfileJobsTitle,
			Description: domain.RunProfileJobsDesc,
			Flag:        domain.FlagJobs,
			Build: func(flow.Answers) (flow.StepContent, error) {
				if len(params.Existing.Jobs) == 0 {
					return flow.StepContent{}, errors.New(domain.RunProfileNoJobsYet)
				}
				return flow.StepContent{Options: jobOptions(params.Existing, params.Initial.Jobs)}, nil
			},
			ValidateSet: func(values []string) error {
				if len(values) == 0 {
					return errors.New(domain.RunProfileJobsRequired)
				}
				return nil
			},
			Resolve:   resolveGivenSet(params.Initial.Jobs),
			Summarize: flow.SummarizeSet,
		},
		{
			Kind:        flow.StepReorder,
			Key:         KeyOrder,
			Label:       domain.RunProfileOrderLabel,
			Title:       domain.RunProfileOrderTitle,
			Description: domain.RunProfileOrderDesc,
			// The order is the start order, so what it offers is exactly what the
			// step before selected — never the whole catalogue again.
			Build: func(answers flow.Answers) (flow.StepContent, error) {
				return flow.StepContent{Options: orderOptions(params.Existing, answers.Values(KeyJobs))}, nil
			},
			Resolve: func(answers flow.Answers) (flow.Answer, error) {
				return flow.Answer{Values: answers.Values(KeyJobs)}, nil
			},
			Summarize: flow.SummarizeSet,
		},
		{
			Kind:        flow.StepSelect,
			Key:         KeyDefault,
			Label:       domain.RunProfileDefaultLabel,
			Title:       domain.RunProfileDefaultTitle,
			Description: defaultDescription(params),
			Options: []flow.Option{
				{Label: domain.RunProfileDefaultNo, Value: domain.RunProfileNoValue},
				{Label: domain.RunProfileDefaultYes, Value: domain.RunProfileYesValue},
			},
			Build: func(flow.Answers) (flow.StepContent, error) {
				return flow.StepContent{Start: boolValue(params.Initial.Default)}, nil
			},
			Resolve: func(flow.Answers) (flow.Answer, error) {
				return flow.Answer{Value: boolValue(params.Initial.Default)}, nil
			},
		},
	}
}

// defaultDescription says which profile the answer would take the default away
// from: only one can hold it, and the reader is entitled to know whose it is.
func defaultDescription(params formParams) string {
	held := rules.FindExistingDefaultProfile(params.Existing, params.ExcludeName)
	if held == "" {
		return domain.RunProfileDefaultDesc
	}
	return domain.RunProfileDefaultDesc + "\n" + fmt.Sprintf(domain.RunProfileDefaultTakenFmt, held)
}

// jobOptions lists the declared jobs, the ones already in the profile first and
// pre-checked: an edit widens a selection rather than rebuilding it.
func jobOptions(cfg domain.RunConfig, selected []string) []flow.Option {
	options := make([]flow.Option, 0, len(cfg.Jobs))
	seen := make(map[string]bool, len(selected))

	for _, name := range selected {
		job, ok := rules.FindJob(cfg, name)
		if !ok {
			continue
		}
		options = append(options, jobOption(job, true))
		seen[job.Name] = true
	}
	for _, job := range cfg.Jobs {
		if seen[job.Name] {
			continue
		}
		options = append(options, jobOption(job, false))
	}
	return options
}

func jobOption(job domain.JobConfig, selected bool) flow.Option {
	return flow.Option{
		Label:    fmt.Sprintf(domain.RunProfileJobOptionFmt, job.Name, job.Kind),
		Value:    job.Name,
		Selected: selected,
	}
}

// orderOptions maps the selected names back to rows, skipping any the config no
// longer declares.
func orderOptions(cfg domain.RunConfig, names []string) []flow.Option {
	options := make([]flow.Option, 0, len(names))
	for _, name := range names {
		job, ok := rules.FindJob(cfg, name)
		if !ok {
			continue
		}
		options = append(options, jobOption(job, true))
	}
	return options
}

func validateName(params formParams) func(string) error {
	return func(value string) error {
		name := strings.TrimSpace(value)
		if name == "" {
			return errors.New(domain.RunProfileNameRequired)
		}
		if strings.ContainsFunc(name, unicode.IsSpace) {
			return errors.New(domain.RunProfileNameSpaces)
		}
		if name == params.ExcludeName {
			return nil
		}
		if _, exists := rules.FindProfile(params.Existing, name); exists {
			return fmt.Errorf(domain.RunProfileExistsFmt, name)
		}
		return nil
	}
}

func resolveGiven(value string) func(flow.Answers) (flow.Answer, error) {
	if value == "" {
		return nil
	}
	return func(flow.Answers) (flow.Answer, error) {
		return flow.Answer{Value: value}, nil
	}
}

func resolveGivenSet(values []string) func(flow.Answers) (flow.Answer, error) {
	if len(values) == 0 {
		return nil
	}
	return func(flow.Answers) (flow.Answer, error) {
		return flow.Answer{Values: values}, nil
	}
}

func boolValue(yes bool) string {
	if yes {
		return domain.RunProfileYesValue
	}
	return domain.RunProfileNoValue
}

func fromAnswers(answers flow.Answers) domain.ProfileConfig {
	return domain.ProfileConfig{
		Name:    strings.TrimSpace(answers.Value(KeyName)),
		Jobs:    answers.Values(KeyOrder),
		Default: answers.Value(KeyDefault) == domain.RunProfileYesValue,
	}
}

func actionStep() flow.Step {
	return flow.Step{
		Kind:  flow.StepSelect,
		Key:   KeyAction,
		Label: domain.RunCRUDActionStepName,
		Build: func(answers flow.Answers) (flow.StepContent, error) {
			return flow.StepContent{
				Title: fmt.Sprintf(domain.RunCRUDActionTitleFmt, answers.Value(target.KeyProfile)),
				Options: []flow.Option{
					{Label: domain.RunCRUDActionEdit, Value: domain.RunCRUDActionEditValue},
					{Label: domain.RunCRUDActionRemove, Value: domain.RunCRUDActionRmValue, Danger: true},
				},
			}, nil
		},
	}
}
