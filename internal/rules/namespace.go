package rules

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
)

type ExpandNamespaceParams struct {
	Namespace domain.JobNamespaceConfig
	Worktree  string
	Ordinal   int
}

type ExpandedNamespace struct {
	Name string
	Env  map[string]string
}

// namespaceToken matches every `{word}`, so one wtm does not define is refused
// rather than reaching a shell as literal braces.
var namespaceToken = regexp.MustCompile(`\{[a-zA-Z_]+\}`)

func ExpandNamespace(params ExpandNamespaceParams) (ExpandedNamespace, error) {
	name, err := expandNamespaceValue(params.Namespace.Name, params)
	if err != nil {
		return ExpandedNamespace{}, err
	}
	if len(params.Namespace.Env) == 0 {
		return ExpandedNamespace{Name: name}, nil
	}

	env := make(map[string]string, len(params.Namespace.Env))
	for key, value := range params.Namespace.Env {
		expanded, expandErr := expandNamespaceValue(value, params)
		if expandErr != nil {
			return ExpandedNamespace{}, expandErr
		}
		env[key] = expanded
	}
	return ExpandedNamespace{Name: name, Env: env}, nil
}

func expandNamespaceValue(value string, params ExpandNamespaceParams) (string, error) {
	replaced := strings.NewReplacer(
		domain.NamespaceTokenWorktree, params.Worktree,
		domain.NamespaceTokenOrdinal, strconv.Itoa(params.Ordinal),
	).Replace(value)

	if leftover := namespaceToken.FindString(replaced); leftover != "" {
		return "", fmt.Errorf("%w: %s", domain.ErrNamespaceUnknownToken, leftover)
	}
	return replaced, nil
}

// NamespaceTokens is what an attach or remove command reads. A namespace whose value
// does not expand yields none: the caller has already been refused at load.
func NamespaceTokens(params ExpandNamespaceParams) map[string]string {
	expanded, err := ExpandNamespace(params)
	if err != nil {
		return nil
	}
	return map[string]string{
		domain.EnvNamespace: expanded.Name,
		domain.EnvWorktree:  params.Worktree,
		domain.EnvOrdinal:   strconv.Itoa(params.Ordinal),
	}
}

func IsShared(job domain.JobConfig) bool { return job.Scope == domain.JobScopeShared }

// HasNamespace reports a block complete enough to act on. An incomplete one is
// refused at load, so a caller reading false here has a job that carves out
// nothing — shared for good, one set of data for every worktree.
func HasNamespace(job domain.JobConfig) bool {
	return job.Namespace != nil && job.Namespace.Name != "" && job.Namespace.Create != ""
}

// ValidateNamespaces is read at load, not at write: a scope or a namespace nobody
// recognizes would otherwise reach a shell, where an unexpanded placeholder
// creates a database literally called "{branch}".
func ValidateNamespaces(cfg domain.RunConfig) []string {
	var errs []string
	for _, job := range cfg.Jobs {
		errs = append(errs, jobScopeErrors(job)...)
	}
	return errs
}

func jobScopeErrors(job domain.JobConfig) []string {
	switch job.Scope {
	case domain.JobScopePerWorktree, domain.JobScopeShared:
	default:
		return []string{fmt.Sprintf(domain.UnknownScopeFmt, job.Name, job.Scope, domain.JobScopeShared)}
	}

	if job.Namespace == nil {
		return nil
	}
	if !IsShared(job) {
		return []string{fmt.Sprintf(domain.NamespaceOnPerWorktreeFmt, job.Name)}
	}
	if !HasNamespace(job) {
		return []string{fmt.Sprintf(domain.NamespaceIncompleteFmt, job.Name)}
	}

	_, err := ExpandNamespace(ExpandNamespaceParams{
		Namespace: *job.Namespace,
		Worktree:  domain.NamespaceProbeWorktree,
	})
	if err != nil {
		return []string{fmt.Sprintf(domain.NamespaceBadTokenFmt, job.Name, err)}
	}
	return nil
}

type SharedJobsUpParams struct {
	Jobs   []domain.JobInfo
	Config domain.RunConfig
}

// SharedJobsUp names the shared services actually running, so a namespace is only
// ever given back to something that can take it.
//
// A claim is deliberately not evidence: it is a worktree's hold on a service,
// and it outlives a daemon restart that left the service itself crashed. The
// worktree being cleaned always holds one, so counting it made the map true for
// every job and had `clean` run DROP DATABASE against a service that was down.
// The config narrows it further: the daemon is machine-wide, and another
// repository's job of the same name says nothing about this one.
func SharedJobsUp(params SharedJobsUpParams) map[string]bool {
	shared := SharedJobNames(params.Config)

	up := map[string]bool{}
	for _, job := range params.Jobs {
		if !shared[job.Name] {
			continue
		}
		if job.Status == domain.JobStatusRunning || job.Status == domain.JobStatusDetached {
			up[job.Name] = true
		}
	}
	return up
}

// JobsHeld narrows a config to the jobs a worktree recorded a namespace in, in the
// config's own order so a recap and a run agree on what they list.
func JobsHeld(cfg domain.RunConfig, held []string) domain.RunConfig {
	if len(held) == 0 {
		return domain.RunConfig{}
	}
	wanted := make(map[string]bool, len(held))
	for _, name := range held {
		wanted[name] = true
	}

	var jobs []domain.JobConfig
	for _, job := range cfg.Jobs {
		if wanted[job.Name] && IsShared(job) {
			jobs = append(jobs, job)
		}
	}
	return domain.RunConfig{Jobs: jobs}
}

// JobsNamed narrows a config to one job, so a caller acting on a single namespace
// hands the removal exactly that one rather than filtering downstream.
func JobsNamed(cfg domain.RunConfig, name string) domain.RunConfig {
	for _, job := range cfg.Jobs {
		if job.Name == name {
			return domain.RunConfig{Jobs: []domain.JobConfig{job}}
		}
	}
	return domain.RunConfig{}
}

type NamespaceEnvParams struct {
	Worktree string
	Ordinal  int
}

// NamespaceEnv rebuilds the little a settled debt needs: the worktree it belonged
// to is gone, so its full environment cannot be resolved any more, and the
// namespace's own name is all that identifies what to give back.
func NamespaceEnv(params NamespaceEnvParams) map[string]string {
	return map[string]string{
		domain.EnvWorktree: params.Worktree,
		domain.EnvOrdinal:  strconv.Itoa(params.Ordinal),
	}
}

// SharedJobNames is what a plan needs to know a link's port does not move: the
// jobs that run once for the repository, by name.
func SharedJobNames(cfg domain.RunConfig) map[string]bool {
	shared := map[string]bool{}
	for _, job := range cfg.Jobs {
		if IsShared(job) {
			shared[job.Name] = true
		}
	}
	return shared
}

// ValidateJobNames refuses a config with two jobs of one name. They share a
// single key in the daemon, so the second is refused as already running, a stop
// is ambiguous, and a shared service's claims cannot tell them apart.
//
// It guards the write path only. A structural error must not block a read: a
// run.toml already holding a duplicate has to stay loadable, or the very
// commands that would let its owner fix it stop working.
func ValidateJobNames(cfg domain.RunConfig) []string {
	seen := map[string]bool{}
	var errs []string
	for _, job := range cfg.Jobs {
		if seen[job.Name] {
			errs = append(errs, fmt.Sprintf(domain.DuplicateJobNameFmt, job.Name))
			continue
		}
		seen[job.Name] = true
	}
	return errs
}

// AnySharedJob is the cheap gate before resolving where shared jobs would run:
// that resolution costs git calls, and most projects declare none.
func AnySharedJob(jobs []domain.JobConfig) bool {
	for _, job := range jobs {
		if IsShared(job) {
			return true
		}
	}
	return false
}

type NamespaceJobsStartedParams struct {
	Jobs []domain.JobConfig
	// Started names the jobs the run left running.
	Started []string
}

// NamespaceJobsStarted narrows a run to the shared jobs that actually came up and
// carve a namespace out. Only those leave anything behind to give back, so only
// those are worth remembering — a job the run never reached created nothing.
func NamespaceJobsStarted(params NamespaceJobsStartedParams) []string {
	if len(params.Started) == 0 {
		return nil
	}
	started := make(map[string]bool, len(params.Started))
	for _, name := range params.Started {
		started[name] = true
	}

	var jobs []string
	for _, job := range params.Jobs {
		if started[job.Name] && IsShared(job) && HasNamespace(job) {
			jobs = append(jobs, job.Name)
		}
	}
	return jobs
}
