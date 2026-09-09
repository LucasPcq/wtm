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

type RemoveNamespacesParams struct {
	Config domain.RunConfig
	// Env is the worktree's own, so a remove command reads the same ports and
	// URLs its attach did.
	Env map[string]string
	// WorkDir is where the command runs. A clean detaches before removing, so
	// the directory is still there.
	WorkDir string
	// Up says which shared jobs are actually running. A namespace cannot be given
	// back to a service that is down, and relighting one to drop a database is
	// worse than deferring it.
	Up map[string]bool
}

// RemoveNamespacesResult is what a clean reports and what it owes. Deferred entries are
// the queue's whole population.
type RemoveNamespacesResult struct {
	Released []domain.NamespaceRef
	Deferred []domain.NamespaceRef
	Errs     []error
}

// RemoveWorktreeNamespaces gives back every namespace this worktree carved out of a shared
// service. wtm never learns what a database or a realm is: it names the namespace
// and runs the command run.toml declares, with the worktree's whole
// environment.
func RemoveWorktreeNamespaces(params RemoveNamespacesParams) RemoveNamespacesResult {
	var result RemoveNamespacesResult
	for _, job := range sharedNamespaces(params.Config) {
		ref := domain.NamespaceRef{
			Job:      job.Name,
			Worktree: params.Env[domain.EnvWorktree],
			Ordinal:  ordinalOf(params.Env),
		}
		if !params.Up[job.Name] {
			result.Deferred = append(result.Deferred, ref)
			continue
		}
		if err := runRemoval(job, params); err != nil {
			result.Errs = append(result.Errs, err)
			result.Deferred = append(result.Deferred, ref)
			continue
		}
		result.Released = append(result.Released, ref)
	}
	return result
}

// sharedNamespaces is every shared job that carves something out, in a stable
// order: a clean reports what it released, and a report that permutes between
// two runs is not one.
func sharedNamespaces(cfg domain.RunConfig) []domain.JobConfig {
	var jobs []domain.JobConfig
	for _, job := range cfg.Jobs {
		if rules.IsShared(job) && rules.HasNamespace(job) && !rules.IsBlankCommand(job.Namespace.Remove) {
			jobs = append(jobs, job)
		}
	}
	sort.Slice(jobs, func(a, b int) bool { return jobs[a].Name < jobs[b].Name })
	return jobs
}

func runRemoval(job domain.JobConfig, params RemoveNamespacesParams) error {
	expand := rules.ExpandNamespaceParams{
		Namespace: *job.Namespace,
		Worktree:  params.Env[domain.EnvWorktree],
		Ordinal:   ordinalOf(params.Env),
	}
	expanded, err := rules.ExpandNamespace(expand)
	if err != nil {
		return fmt.Errorf("job %s: %w", job.Name, err)
	}

	overrides := maps.Clone(params.Env)
	if overrides == nil {
		overrides = map[string]string{}
	}
	maps.Copy(overrides, rules.NamespaceTokens(expand))
	maps.Copy(overrides, expanded.Env)

	spec := rules.ShellCommand(job.Namespace.Remove)
	cmd := exec.Command(spec.Name, spec.Args...)
	cmd.Dir = params.WorkDir
	// Cleared, like every other command wtm runs for a worktree: `wtm prune`
	// typed inside worktree X settles a debt owed by worktree Y, and X's WTM_*
	// and port variables must not reach Y's detach.
	cmd.Env = rules.MergeEnv(rules.MergeEnvParams{
		Env:       os.Environ(),
		Clear:     domain.WorktreeScopedEnv,
		Overrides: overrides,
	})
	if output, runErr := cmd.CombinedOutput(); runErr != nil {
		return fmt.Errorf(domain.NamespaceRemoveFailedFmt, job.Name, expanded.Name,
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
