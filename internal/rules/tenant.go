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
