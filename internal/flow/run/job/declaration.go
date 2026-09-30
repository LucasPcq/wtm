package job

import (
	"errors"
	"maps"
	"slices"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/rules"
)

// declarationSteps are the fields that say how a job relates to the others and
// to the worktrees. Each resolves to what the job already declares — a flag's
// value, else nothing — so an unattended run never invents a relation.
func declarationSteps(params formParams) []flow.Step {
	initial := params.Initial
	return []flow.Step{
		jobSetStep(jobSetParams{
			Key: KeyRuns, Label: domain.RunJobRunsLabel,
			Title: domain.RunJobRunsTitle, Description: domain.RunJobRunsDesc,
			Candidates: otherJobs(params, func(domain.JobConfig) bool { return true }),
			Initial:    initial.Runs, Empty: domain.RunJobRunsSkip,
		}),
		bindsNoPortStep(initial),
		jobSetStep(jobSetParams{
			Key: KeyTouches, Label: domain.RunJobTouchesLabel,
			Title: domain.RunJobTouchesTitle, Description: domain.RunJobTouchesDesc,
			Candidates: otherJobs(params, func(job domain.JobConfig) bool { return job.Kind != domain.JobKindTask }),
			Initial:    initial.Touches, Empty: domain.RunJobTouchesSkip,
		}),
		scopeStep(initial),
		namespaceNameStep(initial),
		namespaceCreateStep(initial),
		namespaceText(namespaceTextParams{
			Key: KeyNamespaceRemove, Label: domain.RunJobNamespaceRemoveLabel,
			Title: domain.RunJobNamespaceRemoveTitle, Description: domain.RunJobNamespaceRemoveDesc,
			Initial: namespaceOf(initial).Remove,
		}),
		namespaceText(namespaceTextParams{
			Key: KeyNamespaceEnv, Label: domain.RunJobNamespaceEnvLabel,
			Title: domain.RunJobNamespaceEnvTitle, Description: domain.RunJobNamespaceEnvDesc,
			Initial: envEntries(namespaceOf(initial).Env),
			Validate: func(value string) error {
				_, err := rules.ParseNamespaceEnv(strings.Fields(value))
				return err
			},
		}),
	}
}

func otherJobs(params formParams, keep func(domain.JobConfig) bool) []string {
	var names []string
	for _, job := range params.Existing.Jobs {
		if job.Name != params.ExcludeName && keep(job) {
			names = append(names, job.Name)
		}
	}
	return names
}

type jobSetParams struct {
	Key         string
	Label       string
	Title       string
	Description string
	Candidates  []string
	Initial     []string
	Empty       string
}

// jobSetStep offers the declared jobs, pre-checked from the declaration. A name
// the declaration carries that is not a candidate is still offered, so opening
// the form never drops it unseen.
func jobSetStep(params jobSetParams) flow.Step {
	candidates := slices.Clone(params.Candidates)
	for _, name := range params.Initial {
		if !slices.Contains(candidates, name) {
			candidates = append(candidates, name)
		}
	}
	return flow.Step{
		Kind:  flow.StepMultiSelect,
		Key:   params.Key,
		Label: params.Label,
		Skip: func(flow.Answers) (bool, string) {
			return len(candidates) == 0, params.Empty
		},
		Build: func(flow.Answers) (flow.StepContent, error) {
			options := make([]flow.Option, 0, len(candidates))
			for _, name := range candidates {
				options = append(options, flow.Option{Label: name, Value: name, Selected: slices.Contains(params.Initial, name)})
			}
			return flow.StepContent{Title: params.Title, Description: params.Description, Options: options}, nil
		},
		Resolve: func(flow.Answers) (flow.Answer, error) {
			return flow.Answer{Values: params.Initial}, nil
		},
		Summarize: flow.SummarizeSet,
	}
}

func bindsNoPortStep(initial domain.JobConfig) flow.Step {
	current := domain.RunJobBindsNoPortNo
	if initial.BindsNoPort {
		current = domain.RunJobBindsNoPortYes
	}
	return flow.Step{
		Kind:        flow.StepSelect,
		Key:         KeyBindsNoPort,
		Label:       domain.RunJobBindsNoPortLabel,
		Title:       domain.RunJobBindsNoPortTitle,
		Description: domain.RunJobBindsNoPortDesc,
		Options: []flow.Option{
			{Label: domain.RunJobBindsNoPortListens, Value: domain.RunJobBindsNoPortNo},
			{Label: domain.RunJobBindsNoPortNone, Value: domain.RunJobBindsNoPortYes},
		},
		Skip: func(answers flow.Answers) (bool, string) {
			if isTask(answers) {
				return true, domain.RunJobBindsNoPortSkipTask
			}
			return strings.TrimSpace(answers.Value(KeyPorts)) != "", domain.RunJobBindsNoPortSkipsPorts
		},
		Build: func(flow.Answers) (flow.StepContent, error) {
			return flow.StepContent{Start: current}, nil
		},
		Resolve: func(flow.Answers) (flow.Answer, error) {
			return flow.Answer{Value: current}, nil
		},
	}
}

func scopeStep(initial domain.JobConfig) flow.Step {
	current := domain.ScopeValuePerWorktree
	if rules.IsShared(initial) {
		current = domain.ScopeValueShared
	}
	return flow.Step{
		Kind:        flow.StepSelect,
		Key:         KeyScope,
		Label:       domain.RunJobScopeLabel,
		Title:       domain.RunJobScopeTitle,
		Description: domain.RunJobScopeDesc,
		Options: []flow.Option{
			{Label: domain.RunJobScopeWorktreeOption, Value: domain.ScopeValuePerWorktree},
			{Label: domain.RunJobScopeSharedOption, Value: domain.ScopeValueShared},
		},
		Skip: func(answers flow.Answers) (bool, string) {
			return isTask(answers), domain.RunJobScopeSkipTask
		},
		Build: func(flow.Answers) (flow.StepContent, error) {
			return flow.StepContent{Start: current}, nil
		},
		Resolve: func(flow.Answers) (flow.Answer, error) {
			return flow.Answer{Value: current}, nil
		},
		Flag: domain.FlagScope,
	}
}

// namespaceNameStep proposes app_{worktree} only to a job that was not shared
// yet: one already shared with no namespace chose one set of data, and an edit
// must not open on a name that would carve one out.
func namespaceNameStep(initial domain.JobConfig) flow.Step {
	proposal := namespaceOf(initial).Name
	if initial.Namespace == nil && !rules.IsShared(initial) {
		proposal = domain.NamespaceNameDefault
	}
	return flow.Step{
		Kind:  flow.StepText,
		Key:   KeyNamespaceName,
		Label: domain.RunJobNamespaceNameLabel,
		Skip:  skipUnlessShared,
		Build: func(flow.Answers) (flow.StepContent, error) {
			return flow.StepContent{
				Title:       domain.RunJobNamespaceNameTitle,
				Description: domain.RunJobNamespaceNameDesc,
				Default:     proposal,
			}, nil
		},
		Validate: func(value string) error {
			if strings.TrimSpace(value) == "" {
				return nil
			}
			_, err := rules.ExpandNamespace(rules.ExpandNamespaceParams{
				Namespace: domain.JobNamespaceConfig{Name: strings.TrimSpace(value)},
				Worktree:  domain.NamespaceProbeWorktree,
			})
			return err
		},
		Resolve: func(flow.Answers) (flow.Answer, error) {
			return flow.Answer{Value: namespaceOf(initial).Name}, nil
		},
		Summarize: func(answer flow.Answer) string {
			if strings.TrimSpace(answer.Value) == "" {
				return domain.RunJobNamespaceNameNone
			}
			return answer.Value
		},
		Flag: domain.FlagNamespaceName,
	}
}

// namespaceCreateStep has no safe default: a name without a create command is
// a block the loader refuses, so an unattended run is refused naming the flag.
func namespaceCreateStep(initial domain.JobConfig) flow.Step {
	create := namespaceOf(initial).Create
	return flow.Step{
		Kind:  flow.StepText,
		Key:   KeyNamespaceCreate,
		Label: domain.RunJobNamespaceCreateLabel,
		Skip:  skipUnlessNamed,
		Build: func(flow.Answers) (flow.StepContent, error) {
			return flow.StepContent{
				Title:       domain.RunJobNamespaceCreateTitle,
				Description: domain.RunJobNamespaceCreateDesc,
				Default:     create,
			}, nil
		},
		Validate: func(value string) error {
			if strings.TrimSpace(value) == "" {
				return errors.New(domain.RunJobNamespaceCreateEmpty)
			}
			return nil
		},
		Resolve: resolveGiven(create),
		Flag:    domain.FlagNamespaceCreate,
	}
}

type namespaceTextParams struct {
	Key         string
	Label       string
	Title       string
	Description string
	Initial     string
	Validate    func(string) error
}

func namespaceText(params namespaceTextParams) flow.Step {
	return flow.Step{
		Kind:  flow.StepText,
		Key:   params.Key,
		Label: params.Label,
		Skip:  skipUnlessNamed,
		Build: func(flow.Answers) (flow.StepContent, error) {
			return flow.StepContent{Title: params.Title, Description: params.Description, Default: params.Initial}, nil
		},
		Validate: params.Validate,
		Resolve: func(flow.Answers) (flow.Answer, error) {
			return flow.Answer{Value: params.Initial}, nil
		},
		Summarize: func(answer flow.Answer) string {
			if strings.TrimSpace(answer.Value) == "" {
				return domain.RunJobStopSummaryNone
			}
			return answer.Value
		},
	}
}

func skipUnlessShared(answers flow.Answers) (bool, string) {
	return answers.Value(KeyScope) != domain.ScopeValueShared, domain.RunJobNamespaceSkip
}

func skipUnlessNamed(answers flow.Answers) (bool, string) {
	if skip, reason := skipUnlessShared(answers); skip {
		return true, reason
	}
	return strings.TrimSpace(answers.Value(KeyNamespaceName)) == "", domain.RunJobNamespaceSkipUnnamed
}

func isTask(answers flow.Answers) bool {
	return domain.JobKind(answers.Value(KeyKind)) == domain.JobKindTask
}

func namespaceOf(job domain.JobConfig) domain.JobNamespaceConfig {
	if job.Namespace == nil {
		return domain.JobNamespaceConfig{}
	}
	return *job.Namespace
}

func envEntries(env map[string]string) string {
	entries := make([]string, 0, len(env))
	for _, key := range slices.Sorted(maps.Keys(env)) {
		entries = append(entries, key+"="+env[key])
	}
	return strings.Join(entries, " ")
}

// withDeclaration reads the declaration steps back. A skipped set step leaves
// the list standing, where an answered empty one withdraws it. A scope nobody
// answered — a task, or an unattended run — keeps the scope and namespace the
// job had, so a namespace passed without --scope shared reaches the loader's
// refusal instead of vanishing.
func withDeclaration(answers flow.Answers, job domain.JobConfig) (domain.JobConfig, error) {
	if answer, ok := answers.Get(KeyRuns); ok && !answer.Skipped {
		job.Runs = answer.Values
	}
	if answer, ok := answers.Get(KeyTouches); ok && !answer.Skipped {
		job.Touches = answer.Values
	}

	switch answer, ok := answers.Get(KeyBindsNoPort); {
	case job.Kind == domain.JobKindTask:
		job.BindsNoPort = false
	case ok && !answer.Skipped:
		job.BindsNoPort = answer.Value == domain.RunJobBindsNoPortYes
	}

	scope, ok := answers.Get(KeyScope)
	if !ok || scope.Skipped || !scope.Asked {
		return job, nil
	}
	if scope.Value != domain.ScopeValueShared {
		job.Scope = domain.JobScopePerWorktree
		job.Namespace = nil
		return job, nil
	}
	job.Scope = domain.JobScopeShared
	name := strings.TrimSpace(answers.Value(KeyNamespaceName))
	if name == "" {
		job.Namespace = nil
		return job, nil
	}
	env, err := rules.ParseNamespaceEnv(strings.Fields(answers.Value(KeyNamespaceEnv)))
	if err != nil {
		return domain.JobConfig{}, err
	}
	job.Namespace = &domain.JobNamespaceConfig{
		Name:   name,
		Create: answers.Value(KeyNamespaceCreate),
		Remove: answers.Value(KeyNamespaceRemove),
		Env:    env,
	}
	return job, nil
}
