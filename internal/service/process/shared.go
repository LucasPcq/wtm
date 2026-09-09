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
	// Membership is not the question — a stopped or crashed job stays in the map
	// so `run logs` can still read it back. Asking it here would have a service
	// answer "already running" for ever once stopped, and post claims onto a
	// corpse.
	if held, ok := m.jobs[ownKey]; ok && rules.IsJobUp(held.Status) {
		m.mu.Unlock()
		return fmt.Errorf("job %s %s", params.Job.Name, domain.JobAlreadyRunningSuffix)
	}
	real, found := m.jobs[realKey]
	up := found && rules.IsJobUp(real.Status)
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

	// The claim is posted before the tenant is carved out, and withdrawn if that
	// fails: a service left running with nothing referencing it is invisible to
	// `run ps` in the worktree that started it, and only a `run down` from the
	// main checkout would ever take it back down.
	m.claim(claimParams{Key: ownKey, Real: realKey, Params: params})
	if err := m.runTenant(tenantParams{Job: params.Job, Env: params.Env, WorkDir: params.WorkDir, Attach: true}); err != nil {
		m.releaseClaim(releaseParams{Key: ownKey, Name: params.Job.Name, Dir: shared.WorkDir})
		return err
	}
	m.persist()
	return nil
}

type releaseParams struct {
	Key  string
	Name string
	Dir  string
}

// releaseClaim withdraws a claim the run could not honour, and takes the
// service with it when nothing else holds it — the state before the start,
// rather than a service nobody can reach.
func (m *Manager) releaseClaim(params releaseParams) {
	m.mu.Lock()
	claim, held := m.jobs[params.Key]
	if held && claim.Status == domain.JobStatusAttached {
		delete(m.jobs, params.Key)
	}
	remaining := m.attachmentsLocked(sharedRef{Name: params.Name, Dir: params.Dir})
	real, found := m.realSharedLocked(sharedRef{Name: params.Name, Dir: params.Dir})
	realRunning := found && real.Status == domain.JobStatusRunning
	m.mu.Unlock()

	m.persist()
	if remaining > 0 || !found {
		return
	}
	_ = m.stopProcess(stopProcessParams{Job: real, Running: realRunning})
}

type claimParams struct {
	Key    string
	Real   string
	Params StartParams
}

// claim posts a worktree's hold on a shared service. The main checkout's own
// hold is the real job — there is no second record to put under the same key,
// and stopShared counts the claims beside it, so the service outlives a `run
// down` there exactly as long as another worktree still holds it.
func (m *Manager) claim(params claimParams) {
	if params.Key == params.Real {
		return
	}
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
		SharedDir: params.Params.Shared.WorkDir,
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
	ref := sharedRef{Name: job.Name, Dir: job.SharedDir}
	if job.Status == domain.JobStatusAttached {
		delete(m.jobs, jobKey(job.Name, job.WorkDir))
	}
	remaining := m.attachmentsLocked(ref)
	real, found := m.realSharedLocked(ref)
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

// sharedRef identifies one shared service: its name and the main checkout it
// runs in. The pair is what keeps two repositories that both declare "db" from
// releasing each other's — the daemon is machine-wide, and a name alone says
// nothing about which repository asked.
type sharedRef struct {
	Name string
	Dir  string
}

// attachmentsLocked counts the claims standing on a shared job, the real job
// excluded: it is the main checkout's own claim, and the whole point is that it
// stops once nobody else holds it.
func (m *Manager) attachmentsLocked(ref sharedRef) int {
	count := 0
	for _, job := range m.jobs {
		if job.Name == ref.Name && job.SharedDir == ref.Dir && job.Status == domain.JobStatusAttached {
			count++
		}
	}
	return count
}

// realSharedLocked is a direct lookup, not a search: the claim carries the very
// key the service is registered under.
func (m *Manager) realSharedLocked(ref sharedRef) (*ManagedJob, bool) {
	if ref.Dir == "" {
		return nil, false
	}
	job, found := m.jobs[jobKey(ref.Name, ref.Dir)]
	if !found || job.Status == domain.JobStatusAttached {
		return nil, false
	}
	return job, true
}

func sharedDirOf(params StartParams) string {
	if !rules.IsShared(params.Job) || params.Shared == nil {
		return ""
	}
	return params.Shared.WorkDir
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
	budget := m.tenantBudget
	if budget <= 0 {
		budget = domain.TenantAttachTimeout
	}
	deadline := time.Now().Add(budget)
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
