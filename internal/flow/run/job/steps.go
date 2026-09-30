package job

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

// The form's answers. The job a command acts on is target.KeyJob, the subject
// every run command names the same way; these are the fields of the declaration
// itself.
const (
	KeyName   = "run.job.name"
	KeyCmd    = "run.job.cmd"
	KeyKind   = "run.job.kind"
	KeyStop   = "run.job.stop"
	KeyCwd    = "run.job.cwd"
	KeyPorts  = "run.job.ports"
	KeyURL    = "run.job.url"
	KeyAction = "run.job.action"

	KeyRuns            = "run.job.runs"
	KeyBindsNoPort     = "run.job.binds_no_port"
	KeyTouches         = "run.job.touches"
	KeyScope           = "run.job.scope"
	KeyNamespaceName   = "run.job.namespace.name"
	KeyNamespaceCreate = "run.job.namespace.create"
	KeyNamespaceRemove = "run.job.namespace.remove"
	KeyNamespaceEnv    = "run.job.namespace.env"
)

type formParams struct {
	Existing domain.RunConfig
	// Initial is what the form opens on: the flags a creation already answered,
	// or the declaration an edit is changing. It is also what every step
	// resolves to when nobody can be asked — a pre-fill and a safe default are
	// the same value seen from two surfaces.
	Initial domain.JobConfig
	// ExcludeName is the name an edit is allowed to keep, since a job is not a
	// duplicate of itself.
	ExcludeName string
}

// formSteps is the job declaration as a session: one step per field, each
// pre-filled with what is already known. A required field with nothing to
// pre-fill has no Resolve, so an unattended run is refused naming its flag
// rather than writing a job nobody described.
func formSteps(params formParams) []flow.Step {
	return append(fieldSteps(params), declarationSteps(params)...)
}

func fieldSteps(params formParams) []flow.Step {
	return []flow.Step{
		{
			Kind:        flow.StepText,
			Key:         KeyName,
			Label:       domain.RunCRUDNameStepName,
			Title:       domain.RunJobNameTitle,
			Description: domain.RunJobNameDesc,
			Default:     params.Initial.Name,
			Arg:         true,
			Validate:    validateName(params),
			Resolve:     resolveGiven(params.Initial.Name),
		},
		{
			Kind:        flow.StepText,
			Key:         KeyCmd,
			Label:       domain.RunJobCmdLabel,
			Title:       domain.RunJobCmdTitle,
			Description: domain.RunJobCmdDesc,
			Default:     params.Initial.Cmd,
			Flag:        domain.FlagCmd,
			Validate: func(value string) error {
				if strings.TrimSpace(value) == "" {
					return errors.New(domain.RunJobCmdRequired)
				}
				return nil
			},
			Resolve: resolveGiven(params.Initial.Cmd),
		},
		{
			Kind:        flow.StepSelect,
			Key:         KeyKind,
			Label:       domain.RunJobKindLabel,
			Title:       domain.RunJobKindTitle,
			Description: domain.RunJobKindDesc,
			Options: []flow.Option{
				{Label: domain.RunJobKindServiceOption, Value: string(domain.JobKindService)},
				{Label: domain.RunJobKindTaskOption, Value: string(domain.JobKindTask)},
			},
			Build: func(flow.Answers) (flow.StepContent, error) {
				return flow.StepContent{Start: string(defaultKind(params.Initial.Kind))}, nil
			},
			Resolve: func(flow.Answers) (flow.Answer, error) {
				return flow.Answer{Value: string(defaultKind(params.Initial.Kind))}, nil
			},
		},
		optionalText(optionalParams{
			Key: KeyStop, Label: domain.RunJobStopLabel,
			Title: domain.RunJobStopTitle, Description: domain.RunJobStopDesc,
			Initial: params.Initial.Stop, None: domain.RunJobStopSummaryNone,
		}),
		optionalText(optionalParams{
			Key: KeyCwd, Label: domain.RunJobCwdLabel,
			Title: domain.RunJobCwdTitle, Description: domain.RunJobCwdDesc,
			Initial: params.Initial.Cwd, None: domain.RunJobCwdSummaryRoot,
		}),
		optionalText(optionalParams{
			Key: KeyPorts, Label: domain.RunJobPortsLabel,
			Title: domain.RunJobPortsTitle, Description: domain.RunJobPortsDesc,
			Initial: strings.Join(rules.PortEntries(params.Initial.Ports), " "),
			None:    domain.RunJobPortsSummaryNone,
			Validate: func(value string) error {
				_, err := rules.ParsePorts(strings.Fields(value))
				return err
			},
		}),
		optionalText(optionalParams{
			Key: KeyURL, Label: domain.RunJobURLLabel,
			Title: domain.RunJobURLTitle, Description: domain.RunJobURLDesc,
			Initial: rules.FormatJobURL(params.Initial.URL),
			None:    domain.RunJobURLSummaryNone,
			Validate: func(value string) error {
				_, err := rules.ParseJobURL(value)
				return err
			},
		}),
	}
}

type optionalParams struct {
	Key         string
	Label       string
	Title       string
	Description string
	Initial     string
	// None is what the recap shows for a field left blank, so a line is never
	// silently empty.
	None     string
	Validate func(string) error
}

func optionalText(params optionalParams) flow.Step {
	return flow.Step{
		Kind:        flow.StepText,
		Key:         params.Key,
		Label:       params.Label,
		Title:       params.Title,
		Description: params.Description,
		Default:     params.Initial,
		Validate:    params.Validate,
		Resolve: func(flow.Answers) (flow.Answer, error) {
			return flow.Answer{Value: params.Initial}, nil
		},
		Summarize: func(answer flow.Answer) string {
			if strings.TrimSpace(answer.Value) == "" {
				return params.None
			}
			return answer.Value
		},
	}
}

// resolveGiven makes a pre-fill the answer of a run that cannot ask, and leaves
// a step with nothing to pre-fill interactive-only — which is what turns a
// missing --cmd into a refusal naming it instead of an empty declaration.
func resolveGiven(value string) func(flow.Answers) (flow.Answer, error) {
	if value == "" {
		return nil
	}
	return func(flow.Answers) (flow.Answer, error) {
		return flow.Answer{Value: value}, nil
	}
}

func validateName(params formParams) func(string) error {
	return func(value string) error {
		name := strings.TrimSpace(value)
		if name == "" {
			return errors.New(domain.RunJobNameRequired)
		}
		if strings.ContainsFunc(name, unicode.IsSpace) {
			return errors.New(domain.RunJobNameSpaces)
		}
		if name == params.ExcludeName {
			return nil
		}
		if _, exists := rules.FindJob(params.Existing, name); exists {
			return fmt.Errorf(domain.RunJobExistsFmt, name)
		}
		return nil
	}
}

func defaultKind(kind domain.JobKind) domain.JobKind {
	if kind == "" {
		return domain.JobKindService
	}
	return kind
}

// fromAnswers reads the form back onto the declaration, so every field the form
// does not show is kept by construction. Rebuilding the job field by field is
// how an edit once unlinked a runner from its children, and later dropped a
// shared service's scope and namespace — each new field was one more to forget.
func fromAnswers(answers flow.Answers, initial domain.JobConfig) (domain.JobConfig, error) {
	ports, err := rules.ParsePorts(strings.Fields(answers.Value(KeyPorts)))
	if err != nil {
		return domain.JobConfig{}, err
	}
	url, err := rules.ParseJobURL(answers.Value(KeyURL))
	if err != nil {
		return domain.JobConfig{}, err
	}
	job := initial
	job.Name = strings.TrimSpace(answers.Value(KeyName))
	job.Cmd = answers.Value(KeyCmd)
	job.Kind = defaultKind(domain.JobKind(answers.Value(KeyKind)))
	job.Stop = answers.Value(KeyStop)
	job.Cwd = answers.Value(KeyCwd)
	job.Ports = ports
	job.URL = url
	return withDeclaration(answers, job)
}

// actionStep is what `run job list` asks once a job is picked: the picker is a
// listing that acts, and this is the acting half.
func actionStep() flow.Step {
	return flow.Step{
		Kind:  flow.StepSelect,
		Key:   KeyAction,
		Label: domain.RunCRUDActionStepName,
		Build: func(answers flow.Answers) (flow.StepContent, error) {
			return flow.StepContent{
				Title: fmt.Sprintf(domain.RunCRUDActionTitleFmt, answers.Value(target.KeyJob)),
				Options: []flow.Option{
					{Label: domain.RunCRUDActionEdit, Value: domain.RunCRUDActionEditValue},
					{Label: domain.RunCRUDActionRemove, Value: domain.RunCRUDActionRmValue, Danger: true},
				},
			}, nil
		},
	}
}
