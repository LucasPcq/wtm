package runjobs

import (
	"context"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"syscall"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

type RemoveNamespacesParams struct {
	Config domain.RunConfig
	// Env is the worktree's own, so a remove command reads the same ports and
	// URLs its attach did.
	Env map[string]string
	// WorkDir is where the command runs. The worktree is gone by the time its
	// data is dropped, so it is a directory that outlives it.
	WorkDir string
	// Up says which shared jobs are actually running. A namespace cannot be given
	// back to a service that is down, and relighting one to drop a database is
	// worse than deferring it.
	Up map[string]bool
	// Timeout bounds each remove command; zero is domain.NamespaceRemoveTimeout.
	Timeout time.Duration
}

type FailedRemoval struct {
	Ref domain.NamespaceRef
	Err error
}

// RemoveNamespacesResult tells a service that was down from a drop that failed
// while it was up: both are owed, but only one of them is the service's fault.
type RemoveNamespacesResult struct {
	Released []domain.NamespaceRef
	Down     []domain.NamespaceRef
	Failed   []FailedRemoval
}

// Deferred is everything still owed: the queue's whole population.
func (r RemoveNamespacesResult) Deferred() []domain.NamespaceRef {
	deferred := append([]domain.NamespaceRef{}, r.Down...)
	for _, failed := range r.Failed {
		deferred = append(deferred, failed.Ref)
	}
	return deferred
}

// RemoveWorktreeNamespaces gives back every namespace this worktree carved out of a shared
// service. wtm never learns what a database or a realm is: it names the namespace
// and runs the command run.toml declares, with the worktree's whole
// environment.
func RemoveWorktreeNamespaces(ctx context.Context, params RemoveNamespacesParams) RemoveNamespacesResult {
	var result RemoveNamespacesResult
	for _, job := range sharedNamespaces(params.Config) {
		ref := domain.NamespaceRef{
			Job:      job.Name,
			Worktree: params.Env[domain.EnvWorktree],
			Ordinal:  ordinalOf(params.Env),
		}
		if !params.Up[job.Name] {
			result.Down = append(result.Down, ref)
			continue
		}
		if err := runRemoval(ctx, job, params); err != nil {
			result.Failed = append(result.Failed, FailedRemoval{Ref: ref, Err: err})
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

func runRemoval(parent context.Context, job domain.JobConfig, params RemoveNamespacesParams) error {
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

	timeout := params.Timeout
	if timeout <= 0 {
		timeout = domain.NamespaceRemoveTimeout
	}
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()

	spec := rules.ShellCommand(job.Namespace.Remove)
	cmd := exec.CommandContext(ctx, spec.Name, spec.Args...)
	cmd.Dir = params.WorkDir
	// The whole group goes on a timeout: killing the shell alone leaves its
	// psql holding the output pipe, and the read would wait on it regardless.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
	cmd.WaitDelay = domain.NamespaceRemoveKillGrace
	// Cleared, like every other command wtm runs for a worktree: `wtm prune`
	// typed inside worktree X settles a debt owed by worktree Y, and X's WTM_*
	// and port variables must not reach Y's detach.
	cmd.Env = rules.MergeEnv(rules.MergeEnvParams{
		Env:       os.Environ(),
		Clear:     domain.WorktreeScopedEnv,
		Overrides: overrides,
	})
	output, runErr := cmd.CombinedOutput()
	if err := parent.Err(); err != nil {
		return fmt.Errorf(domain.NamespaceRemoveFailedFmt, job.Name, expanded.Name, err)
	}
	if ctx.Err() != nil {
		return fmt.Errorf(domain.NamespaceRemoveFailedFmt, job.Name, expanded.Name,
			fmt.Errorf(domain.NamespaceRemoveTimedOutFmt, timeout))
	}
	if runErr != nil {
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
