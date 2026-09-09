package rules

import "github.com/LucasPcq/wtm/internal/domain"

// ServiceScopeChoice is one compose service as the scope step lists it: what it
// is, what wtm proposes, and — when the proposal is not a question at all — why.
type ServiceScopeChoice struct {
	File    string
	Service string
	Image   string
	Scope   domain.JobScope
	// Fixed says the answer is not the reader's to give. Reason says why.
	Fixed     bool
	Reason    string
	Namespace *domain.JobNamespaceConfig
}

type ServiceScopeChoicesParams struct {
	// Scans are the selected files' contents, keyed by file.
	Scans map[string]domain.ComposeScan
	// Files is the selection, in the order the step lists them.
	Files []string
	// Existing is run.toml as it stands. Where it has an opinion it wins over
	// detection, which is what makes a re-init show what was decided last time
	// rather than what a fresh detection would guess.
	Existing domain.RunConfig
}

// ServiceScopeChoices is the complete list the step shows — never a subset, so
// a re-init cannot silently drop a service the reader had already answered for.
func ServiceScopeChoices(params ServiceScopeChoicesParams) []ServiceScopeChoice {
	var choices []ServiceScopeChoice
	for _, file := range params.Files {
		for _, service := range params.Scans[file].Services {
			choices = append(choices, scopeChoiceFor(scopeChoiceParams{
				File: file, Service: service, Existing: params.Existing,
			}))
		}
	}
	return choices
}

type scopeChoiceParams struct {
	File     string
	Service  domain.ComposeService
	Existing domain.RunConfig
}

func scopeChoiceFor(params scopeChoiceParams) ServiceScopeChoice {
	choice := ServiceScopeChoice{
		File:    params.File,
		Service: params.Service.Name,
		Image:   params.Service.Image,
	}

	// Structural, not a guess about the image's name: a service compiled from
	// the source of this worktree serves this worktree's code, so sharing it
	// would serve one worktree's build to all of them.
	if params.Service.HasBuild {
		choice.Fixed, choice.Reason = true, domain.ScopeReasonBuild
		return choice
	}

	// The config outranks detection wherever it speaks: a re-init must show what
	// was decided, not what a fresh look would propose.
	if job, found := existingSharedJob(params.Existing, params.Service.Name); found {
		choice.Scope, choice.Namespace = domain.JobScopeShared, job.Namespace
		return choice
	}
	if declaredPerWorktree(params.Existing, params.Service.Name) {
		return choice
	}

	return choice
}

func existingSharedJob(cfg domain.RunConfig, name string) (domain.JobConfig, bool) {
	for _, job := range cfg.Jobs {
		if job.Name == name && IsShared(job) {
			return job, true
		}
	}
	return domain.JobConfig{}, false
}

func declaredPerWorktree(cfg domain.RunConfig, name string) bool {
	for _, job := range cfg.Jobs {
		if job.Name == name {
			return true
		}
	}
	return false
}

// SharedFromChoices is what the step's answers become for the job builder.
func SharedFromChoices(choices []ServiceScopeChoice) []domain.SharedComposeService {
	var shared []domain.SharedComposeService
	for _, choice := range choices {
		if choice.Fixed || choice.Scope != domain.JobScopeShared {
			continue
		}
		shared = append(shared, domain.SharedComposeService{
			File: choice.File, Service: choice.Service, Namespace: choice.Namespace,
		})
	}
	return shared
}

// AnyScopeAnswerable says the step has a question to put at all: a file whose
// services are every one of them built here settles itself.
func AnyScopeAnswerable(choices []ServiceScopeChoice) bool {
	for _, choice := range choices {
		if !choice.Fixed {
			return true
		}
	}
	return false
}

// ScopesSkipReason explains a step that never ran, so a recap says why rather
// than leaving a gap.
func ScopesSkipReason(services int) string {
	if services == 0 {
		return domain.ScopesSkipNoServices
	}
	return domain.ScopesSkipAllBuilt
}

type SharedFromConfigParams struct {
	Existing domain.RunConfig
	Scans    map[string]domain.ComposeScan
	Files    []string
}

// SharedFromConfig recovers what run.toml already declares shared, for a run
// that never put the question — a non-interactive init, or one whose step was
// skipped. Without it a re-init would quietly un-share every service, which is
// the write-side of the same rule that keeps a flag from erasing a recap line.
func SharedFromConfig(params SharedFromConfigParams) []domain.SharedComposeService {
	var shared []domain.SharedComposeService
	for _, file := range params.Files {
		for _, service := range params.Scans[file].Services {
			job, found := existingSharedJob(params.Existing, service.Name)
			if !found {
				continue
			}
			shared = append(shared, domain.SharedComposeService{
				File: file, Service: service.Name, Namespace: job.Namespace,
			})
		}
	}
	return shared
}
