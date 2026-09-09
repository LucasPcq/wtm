package runjobs

import (
	"fmt"
	"maps"
	"os"
	"os/exec"
	"sort"
	"strconv"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

type DetachParams struct {
	Config domain.RunConfig
	// Env is the worktree's own, so a detach command reads the same ports and
	// URLs its attach did.
	Env map[string]string
	// WorkDir is where the command runs. A clean detaches before removing, so
	// the directory is still there.
	WorkDir string
	// Up says which shared jobs are actually running. A tenant cannot be given
	// back to a service that is down, and relighting one to drop a database is
	// worse than deferring it.
	Up map[string]bool
}

// DetachResult is what a clean reports and what it owes. Deferred entries are
// the queue's whole population.
type DetachResult struct {
	Released []domain.TenantRef
	Deferred []domain.TenantRef
	Errs     []error
}

// DetachWorktree gives back every tenant this worktree carved out of a shared
// service. wtm never learns what a database or a realm is: it names the tenant
// and runs the command run.toml declares, with the worktree's whole
// environment.
func DetachWorktree(params DetachParams) DetachResult {
	var result DetachResult
	for _, job := range sharedTenants(params.Config) {
		ref := domain.TenantRef{
			Job:      job.Name,
			Worktree: params.Env[domain.EnvWorktree],
			Ordinal:  ordinalOf(params.Env),
		}
		if !params.Up[job.Name] {
			result.Deferred = append(result.Deferred, ref)
			continue
		}
		if err := runDetach(job, params); err != nil {
			result.Errs = append(result.Errs, err)
			result.Deferred = append(result.Deferred, ref)
			continue
		}
		result.Released = append(result.Released, ref)
	}
	return result
}

// sharedTenants is every shared job that carves something out, in a stable
// order: a clean reports what it released, and a report that permutes between
// two runs is not one.
func sharedTenants(cfg domain.RunConfig) []domain.JobConfig {
	var jobs []domain.JobConfig
	for _, job := range cfg.Jobs {
		if rules.IsShared(job) && rules.HasTenant(job) && !rules.IsBlankCommand(job.Tenant.Detach) {
			jobs = append(jobs, job)
		}
	}
	sort.Slice(jobs, func(a, b int) bool { return jobs[a].Name < jobs[b].Name })
	return jobs
}

func runDetach(job domain.JobConfig, params DetachParams) error {
	expand := rules.ExpandTenantParams{
		Tenant:   *job.Tenant,
		Worktree: params.Env[domain.EnvWorktree],
		Ordinal:  ordinalOf(params.Env),
	}
	expanded, err := rules.ExpandTenant(expand)
	if err != nil {
		return fmt.Errorf("job %s: %w", job.Name, err)
	}

	overrides := maps.Clone(params.Env)
	if overrides == nil {
		overrides = map[string]string{}
	}
	maps.Copy(overrides, rules.TenantTokens(expand))
	maps.Copy(overrides, expanded.Env)

	spec := rules.ShellCommand(job.Tenant.Detach)
	cmd := exec.Command(spec.Name, spec.Args...)
	cmd.Dir = params.WorkDir
	cmd.Env = rules.MergeEnv(rules.MergeEnvParams{Env: os.Environ(), Overrides: overrides})
	if output, runErr := cmd.CombinedOutput(); runErr != nil {
		return fmt.Errorf(domain.TenantDetachFailedFmt, job.Name, expanded.Name,
			fmt.Errorf("%w: %s", runErr, rules.SanitizeLogLine(string(output))))
	}
	return nil
}

func ordinalOf(env map[string]string) int {
	ordinal, err := strconv.Atoi(env[domain.EnvOrdinal])
	if err != nil {
		return 0
	}
	return ordinal
}
