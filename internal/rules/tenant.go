package rules

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
)

type ExpandTenantParams struct {
	Tenant   domain.JobTenantConfig
	Worktree string
	Ordinal  int
}

// ExpandedTenant is one worktree's slice of a shared service, ready to be
// handed to a shell: the tenant's resolved name, and the variables the
// worktree's own .env receives.
type ExpandedTenant struct {
	Name string
	Env  map[string]string
}

// tenantToken matches every `{word}`, so one wtm does not define is refused
// rather than reaching a shell as literal braces.
var tenantToken = regexp.MustCompile(`\{[a-zA-Z_]+\}`)

func ExpandTenant(params ExpandTenantParams) (ExpandedTenant, error) {
	name, err := expandTenantValue(params.Tenant.Name, params)
	if err != nil {
		return ExpandedTenant{}, err
	}
	if len(params.Tenant.Env) == 0 {
		return ExpandedTenant{Name: name}, nil
	}

	env := make(map[string]string, len(params.Tenant.Env))
	for key, value := range params.Tenant.Env {
		expanded, expandErr := expandTenantValue(value, params)
		if expandErr != nil {
			return ExpandedTenant{}, expandErr
		}
		env[key] = expanded
	}
	return ExpandedTenant{Name: name, Env: env}, nil
}

func expandTenantValue(value string, params ExpandTenantParams) (string, error) {
	replaced := strings.NewReplacer(
		domain.TenantTokenWorktree, params.Worktree,
		domain.TenantTokenOrdinal, strconv.Itoa(params.Ordinal),
	).Replace(value)

	if leftover := tenantToken.FindString(replaced); leftover != "" {
		return "", fmt.Errorf("%w: %s", domain.ErrTenantUnknownToken, leftover)
	}
	return replaced, nil
}

// TenantTokens is what an attach or detach command reads. A tenant whose value
// does not expand yields none: the caller has already been refused at load.
func TenantTokens(params ExpandTenantParams) map[string]string {
	expanded, err := ExpandTenant(params)
	if err != nil {
		return nil
	}
	return map[string]string{
		domain.EnvTenant:   expanded.Name,
		domain.EnvWorktree: params.Worktree,
		domain.EnvOrdinal:  strconv.Itoa(params.Ordinal),
	}
}

func IsShared(job domain.JobConfig) bool { return job.Scope == domain.JobScopeShared }

// HasTenant reports a block complete enough to act on. An incomplete one is
// refused at load, so a caller reading false here has a job that carves out
// nothing — shared for good, one set of data for every worktree.
func HasTenant(job domain.JobConfig) bool {
	return job.Tenant != nil && job.Tenant.Name != "" && job.Tenant.Attach != ""
}

// ValidateTenants is read at load, not at write: a scope or a tenant nobody
// recognizes would otherwise reach a shell, where an unexpanded placeholder
// creates a database literally called "{branch}".
func ValidateTenants(cfg domain.RunConfig) []string {
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

	if job.Tenant == nil {
		return nil
	}
	if !IsShared(job) {
		return []string{fmt.Sprintf(domain.TenantOnPerWorktreeFmt, job.Name)}
	}
	if !HasTenant(job) {
		return []string{fmt.Sprintf(domain.TenantIncompleteFmt, job.Name)}
	}

	_, err := ExpandTenant(ExpandTenantParams{
		Tenant:   *job.Tenant,
		Worktree: domain.TenantProbeWorktree,
	})
	if err != nil {
		return []string{fmt.Sprintf(domain.TenantBadTokenFmt, job.Name, err)}
	}
	return nil
}

type SharedJobsUpParams struct {
	Jobs   []domain.JobInfo
	Config domain.RunConfig
}

// SharedJobsUp names the shared services actually running, so a tenant is only
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

// JobsHeld narrows a config to the jobs a worktree recorded a tenant in, in the
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

// JobsNamed narrows a config to one job, so a caller acting on a single tenant
// hands the detach exactly that one rather than filtering downstream.
func JobsNamed(cfg domain.RunConfig, name string) domain.RunConfig {
	for _, job := range cfg.Jobs {
		if job.Name == name {
			return domain.RunConfig{Jobs: []domain.JobConfig{job}}
		}
	}
	return domain.RunConfig{}
}

type TenantEnvParams struct {
	Worktree string
	Ordinal  int
}

// TenantEnv rebuilds the little a settled debt needs: the worktree it belonged
// to is gone, so its full environment cannot be resolved any more, and the
// tenant's own name is all that identifies what to give back.
func TenantEnv(params TenantEnvParams) map[string]string {
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

type TenantJobsStartedParams struct {
	Jobs []domain.JobConfig
	// Started names the jobs the run left running.
	Started []string
}

// TenantJobsStarted narrows a run to the shared jobs that actually came up and
// carve a tenant out. Only those leave anything behind to give back, so only
// those are worth remembering — a job the run never reached created nothing.
func TenantJobsStarted(params TenantJobsStartedParams) []string {
	if len(params.Started) == 0 {
		return nil
	}
	started := make(map[string]bool, len(params.Started))
	for _, name := range params.Started {
		started[name] = true
	}

	var jobs []string
	for _, job := range params.Jobs {
		if started[job.Name] && IsShared(job) && HasTenant(job) {
			jobs = append(jobs, job.Name)
		}
	}
	return jobs
}
