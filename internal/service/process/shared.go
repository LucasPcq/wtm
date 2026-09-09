package process

import (
	"fmt"
	"maps"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

// startShared runs a shared job once for the repository and records the asking
// worktree's claim on it. The real job lives under the main checkout's key —
// jobKey is untouched, the sharing is entirely in the choice of work dir — and
// every other worktree posts an attachment beside it. That makes the job table
// itself the reference count, with no second registry to keep in step.
func (m *Manager) startShared(params StartParams) error {
	shared := params.Shared
	if shared == nil || shared.WorkDir == "" {
		return fmt.Errorf(domain.SharedNoContextFmt, params.Job.Name, domain.ErrNoMainCheckout)
	}

	ownKey := jobKey(params.Job.Name, params.WorkDir)
	realKey := jobKey(params.Job.Name, shared.WorkDir)

	m.mu.Lock()
	if _, held := m.jobs[ownKey]; held {
		m.mu.Unlock()
		return fmt.Errorf("job %s %s", params.Job.Name, domain.JobAlreadyRunningSuffix)
	}
	_, up := m.jobs[realKey]
	m.mu.Unlock()

	if !up {
		real := params
		real.WorkDir = shared.WorkDir
		real.Env = shared.Env
		real.LogDir = shared.LogDir
		real.real = true
		// Losing the race to another worktree starting the same service is a
		// success, not a failure: `run up --all` fans out over worktrees, and
		// two of them reaching here at once must not fail one whole run.
		if err := m.Start(real); err != nil && !strings.HasSuffix(err.Error(), domain.JobAlreadyRunningSuffix) {
			return err
		}
	}

	if err := m.runTenant(tenantParams{Job: params.Job, Env: params.Env, WorkDir: params.WorkDir, Attach: true}); err != nil {
		return err
	}

	// The main checkout's own claim is the real job: posting a second record
	// under the same key is impossible, and unnecessary — stopShared counts the
	// attachments beside it, so the service outlives a `run down` there exactly
	// as long as another worktree still holds it.
	if ownKey == realKey {
		return nil
	}
	m.attach(attachParams{Key: ownKey, Params: params})
	m.persist()
	return nil
}

type attachParams struct {
	Key    string
	Params StartParams
}

func (m *Manager) attach(params attachParams) {
	exited := make(chan struct{})
	close(exited)

	m.mu.Lock()
	m.jobs[params.Key] = &ManagedJob{
		Name:      params.Params.Job.Name,
		Config:    params.Params.Job,
		Status:    domain.JobStatusAttached,
		WorkDir:   params.Params.WorkDir,
		StartedAt: time.Now(),
		Env:       params.Params.Env,
		LogDir:    params.Params.LogDir,
		exited:    exited,
	}
	m.mu.Unlock()
}

// stopShared releases one worktree's claim and stops the service only once no
// claim is left anywhere. The tenant is deliberately not detached: stopping is
// not destroying, and a `run down` that dropped a database would make the
// command unusable.
func (m *Manager) stopShared(job *ManagedJob) error {
	m.mu.Lock()
	if job.Status == domain.JobStatusAttached {
		delete(m.jobs, jobKey(job.Name, job.WorkDir))
	}
	remaining := m.attachmentsLocked(job.Name)
	real, found := m.realSharedLocked(job.Name)
	// Snapshotted under the lock, like stopByKey does: Status is written by the
	// goroutine that reaps the process, and reading it outside is a race.
	realRunning := found && real.Status == domain.JobStatusRunning
	m.mu.Unlock()

	m.persist()

	if remaining > 0 || !found {
		return nil
	}
	// The tear-down itself, never stopByKey: that would come straight back here
	// and find the same zero claims, for ever.
	return m.stopProcess(stopProcessParams{Job: real, Running: realRunning})
}

// attachmentsLocked counts the claims standing on a shared job, the real job
// excluded: it is the main checkout's own claim, and the whole point is that it
// stops once nobody else holds it.
func (m *Manager) attachmentsLocked(name string) int {
	count := 0
	for _, job := range m.jobs {
		if job.Name == name && job.Status == domain.JobStatusAttached {
			count++
		}
	}
	return count
}

func (m *Manager) realSharedLocked(name string) (*ManagedJob, bool) {
	for _, job := range m.jobs {
		if job.Name == name && job.Status != domain.JobStatusAttached {
			return job, true
		}
	}
	return nil, false
}

type tenantParams struct {
	Job     domain.JobConfig
	Env     map[string]string
	WorkDir string
	Attach  bool
}

// runTenant carves out — or gives back — this worktree's slice of a shared
// service. wtm never learns what a database or a realm is: it names the tenant
// and hands the user's own command the worktree's whole environment, ports and
// URLs included, which is what lets a keycloak realm's redirect URIs point at
// the fronts of the worktree asking.
func (m *Manager) runTenant(params tenantParams) error {
	if !rules.HasTenant(params.Job) {
		return nil
	}
	line := params.Job.Tenant.Attach
	if !params.Attach {
		line = params.Job.Tenant.Detach
	}
	if rules.IsBlankCommand(line) {
		return nil
	}

	expand := rules.ExpandTenantParams{
		Tenant:   *params.Job.Tenant,
		Worktree: params.Env[domain.EnvWorktree],
		Ordinal:  ordinalOf(params.Env),
	}
	expanded, err := rules.ExpandTenant(expand)
	if err != nil {
		return fmt.Errorf("job %s: %w", params.Job.Name, err)
	}

	// Copied rather than written through: WithPortEnv hands back the very map it
	// was given when the job declares no port, and that map is the job's own.
	overrides := maps.Clone(withJobPorts(params.Job, params.Env))
	if overrides == nil {
		overrides = map[string]string{}
	}
	for key, value := range rules.TenantTokens(expand) {
		overrides[key] = value
	}
	for key, value := range expanded.Env {
		overrides[key] = value
	}
	env := jobEnv(jobEnvParams{Kind: domain.JobKindTask, Overrides: overrides})

	last := m.tenantAttempt(tenantAttemptParams{Line: line, Dir: params.WorkDir, Env: env, Retry: params.Attach})
	if last == nil {
		return nil
	}
	format := domain.TenantDetachFailedFmt
	if params.Attach {
		format = domain.TenantAttachFailedFmt
	}
	return fmt.Errorf(format, params.Job.Name, expanded.Name, last)
}

type tenantAttemptParams struct {
	Line  string
	Dir   string
	Env   []string
	Retry bool
}

// tenantAttempt retries an attach within a budget: the service it talks to was
// started moments ago, so a first refusal means "postgres is not accepting
// connections yet" far more often than it means the command is wrong. A detach
// runs against a service already up, so it is asked exactly once.
func (m *Manager) tenantAttempt(params tenantAttemptParams) error {
	deadline := time.Now().Add(domain.TenantAttachTimeout)
	for {
		err := runTenantCommand(params)
		if err == nil {
			return nil
		}
		if !params.Retry || time.Now().After(deadline) {
			return err
		}
		time.Sleep(domain.TenantAttachInterval)
	}
}

func runTenantCommand(params tenantAttemptParams) error {
	spec := rules.ShellCommand(params.Line)
	cmd := exec.Command(spec.Name, spec.Args...)
	cmd.Dir = params.Dir
	cmd.Env = params.Env
	output, err := cmd.CombinedOutput()
	if err == nil {
		return nil
	}
	if len(output) == 0 {
		return err
	}
	return fmt.Errorf("%w: %s", err, rules.SanitizeLogLine(string(output)))
}

func ordinalOf(env map[string]string) int {
	ordinal, err := strconv.Atoi(env[domain.EnvOrdinal])
	if err != nil {
		return 0
	}
	return ordinal
}
