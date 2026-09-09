package process

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
)

type sharedFixture struct {
	manager *Manager
	main    string
	first   string
	second  string
	job     domain.JobConfig
	// witness is where a tenant command appends the tenant it was handed, so a
	// test reads what actually ran rather than trusting a return value.
	witness string
}

func newSharedFixture(t *testing.T, tenant *domain.JobTenantConfig) sharedFixture {
	t.Helper()
	root := t.TempDir()
	fixture := sharedFixture{
		manager: NewManagerWith(ManagerParams{TenantBudget: 50 * time.Millisecond}),
		main:    filepath.Join(root, "main"),
		first:   filepath.Join(root, "feat-a"),
		second:  filepath.Join(root, "feat-b"),
		witness: filepath.Join(root, "witness"),
	}
	for _, dir := range []string{fixture.main, fixture.first, fixture.second} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
	}
	fixture.job = domain.JobConfig{
		Name: "db", Kind: domain.JobKindService, Cmd: "sleep 30",
		Scope: domain.JobScopeShared, Tenant: tenant,
	}
	t.Cleanup(func() { _ = fixture.manager.StopAll() })
	return fixture
}

func (f sharedFixture) start(t *testing.T, workDir, worktree string) error {
	t.Helper()
	return f.manager.Start(StartParams{
		Job:     f.job,
		WorkDir: workDir,
		Env:     map[string]string{domain.EnvWorktree: worktree, domain.EnvOrdinal: "1"},
		Shared: &domain.SharedJobContext{
			WorkDir: f.main,
			Env:     map[string]string{domain.EnvWorktree: "main", domain.EnvOrdinal: "0"},
		},
	})
}

func (f sharedFixture) statusIn(workDir string) (domain.JobStatus, bool) {
	f.manager.mu.Lock()
	defer f.manager.mu.Unlock()
	job, ok := f.manager.jobs[jobKey("db", workDir)]
	if !ok {
		return "", false
	}
	return job.Status, true
}

func (f sharedFixture) processes() int {
	f.manager.mu.Lock()
	defer f.manager.mu.Unlock()
	count := 0
	for _, job := range f.manager.jobs {
		if job.Cmd != nil {
			count++
		}
	}
	return count
}

func (f sharedFixture) witnessed(t *testing.T) string {
	t.Helper()
	content, err := os.ReadFile(f.witness)
	if os.IsNotExist(err) {
		return ""
	}
	if err != nil {
		t.Fatalf("read witness: %v", err)
	}
	return string(content)
}

// A shared job started from a worktree runs under the main checkout, and the
// worktree that asked holds nothing but a claim.
func TestStartSharedRunsUnderMainCheckout(t *testing.T) {
	f := newSharedFixture(t, nil)

	if err := f.start(t, f.first, "feat_a"); err != nil {
		t.Fatalf("start: %v", err)
	}

	if status, ok := f.statusIn(f.main); !ok || status != domain.JobStatusRunning {
		t.Errorf("main checkout status = %q (found %v), want running", status, ok)
	}
	if status, ok := f.statusIn(f.first); !ok || status != domain.JobStatusAttached {
		t.Errorf("worktree status = %q (found %v), want attached", status, ok)
	}
	if got := f.processes(); got != 1 {
		t.Errorf("processes = %d, want 1", got)
	}
}

func TestStartSharedSecondWorktreeSpawnsNothing(t *testing.T) {
	f := newSharedFixture(t, nil)

	if err := f.start(t, f.first, "feat_a"); err != nil {
		t.Fatalf("first start: %v", err)
	}
	if err := f.start(t, f.second, "feat_b"); err != nil {
		t.Fatalf("second start: %v", err)
	}

	if got := f.processes(); got != 1 {
		t.Errorf("processes = %d, want 1: a shared service runs once", got)
	}
	if status, _ := f.statusIn(f.second); status != domain.JobStatusAttached {
		t.Errorf("second worktree status = %q, want attached", status)
	}
}

func TestStartSharedRefusesASecondClaimFromTheSameWorktree(t *testing.T) {
	f := newSharedFixture(t, nil)

	if err := f.start(t, f.first, "feat_a"); err != nil {
		t.Fatalf("first start: %v", err)
	}
	err := f.start(t, f.first, "feat_a")
	if err == nil || !strings.HasSuffix(err.Error(), domain.JobAlreadyRunningSuffix) {
		t.Errorf("err = %v, want one ending in %q", err, domain.JobAlreadyRunningSuffix)
	}
}

// Started from the main checkout itself, the real job is that checkout's own
// claim: there is no second record to post under the same key.
func TestStartSharedFromTheMainCheckoutPostsNoAttachment(t *testing.T) {
	f := newSharedFixture(t, nil)

	if err := f.start(t, f.main, "main"); err != nil {
		t.Fatalf("start: %v", err)
	}
	if status, _ := f.statusIn(f.main); status != domain.JobStatusRunning {
		t.Errorf("status = %q, want running", status)
	}
	if got := len(f.manager.List()); got != 1 {
		t.Errorf("records = %d, want 1", got)
	}
}

func TestStopSharedKeepsTheServiceWhileAnotherWorktreeHoldsIt(t *testing.T) {
	f := newSharedFixture(t, nil)

	if err := f.start(t, f.first, "feat_a"); err != nil {
		t.Fatalf("first start: %v", err)
	}
	if err := f.start(t, f.second, "feat_b"); err != nil {
		t.Fatalf("second start: %v", err)
	}

	if err := f.manager.Stop("db", f.first); err != nil {
		t.Fatalf("stop: %v", err)
	}

	if _, ok := f.statusIn(f.first); ok {
		t.Error("the released worktree still holds a record")
	}
	if status, _ := f.statusIn(f.main); status != domain.JobStatusRunning {
		t.Errorf("service status = %q, want running: feat-b still holds it", status)
	}
}

func TestStopSharedStopsTheServiceWithTheLastClaim(t *testing.T) {
	f := newSharedFixture(t, nil)

	if err := f.start(t, f.first, "feat_a"); err != nil {
		t.Fatalf("first start: %v", err)
	}
	if err := f.start(t, f.second, "feat_b"); err != nil {
		t.Fatalf("second start: %v", err)
	}
	if err := f.manager.Stop("db", f.first); err != nil {
		t.Fatalf("stop first: %v", err)
	}
	if err := f.manager.Stop("db", f.second); err != nil {
		t.Fatalf("stop second: %v", err)
	}

	if status, _ := f.statusIn(f.main); status != domain.JobStatusStopped {
		t.Errorf("service status = %q, want stopped: nobody holds it any more", status)
	}
}

// The main checkout's own claim is the real job, so a `run down` there while
// another worktree holds the service must not take it down.
func TestStopSharedFromTheMainCheckoutSpareTheServiceWhileHeld(t *testing.T) {
	f := newSharedFixture(t, nil)

	if err := f.start(t, f.main, "main"); err != nil {
		t.Fatalf("main start: %v", err)
	}
	if err := f.start(t, f.first, "feat_a"); err != nil {
		t.Fatalf("worktree start: %v", err)
	}

	if err := f.manager.Stop("db", f.main); err != nil {
		t.Fatalf("stop: %v", err)
	}

	if status, _ := f.statusIn(f.main); status != domain.JobStatusRunning {
		t.Errorf("service status = %q, want running: feat-a still holds it", status)
	}
}

func TestSharedTenantAttachesOncePerWorktree(t *testing.T) {
	f := newSharedFixture(t, &domain.JobTenantConfig{
		Name:   "crm_{worktree}",
		Attach: "printf '%s\\n' \"$WTM_TENANT\" >> " + "WITNESS",
	})
	f.job.Tenant.Attach = strings.Replace(f.job.Tenant.Attach, "WITNESS", f.witness, 1)

	if err := f.start(t, f.first, "feat_a"); err != nil {
		t.Fatalf("first start: %v", err)
	}
	if err := f.start(t, f.second, "feat_b"); err != nil {
		t.Fatalf("second start: %v", err)
	}

	got := f.witnessed(t)
	if got != "crm_feat_a\ncrm_feat_b\n" {
		t.Errorf("attach ran with %q, want one line per worktree", got)
	}
}

// Stopping is not destroying: a `run down` that dropped a database would make
// the command unusable.
func TestSharedStopNeverDetaches(t *testing.T) {
	f := newSharedFixture(t, &domain.JobTenantConfig{
		Name:   "crm_{worktree}",
		Attach: "true",
		Detach: "printf 'detached\\n' >> " + "WITNESS",
	})
	f.job.Tenant.Detach = strings.Replace(f.job.Tenant.Detach, "WITNESS", f.witness, 1)

	if err := f.start(t, f.first, "feat_a"); err != nil {
		t.Fatalf("start: %v", err)
	}
	if err := f.manager.Stop("db", f.first); err != nil {
		t.Fatalf("stop: %v", err)
	}

	if got := f.witnessed(t); got != "" {
		t.Errorf("detach ran on stop (%q); it belongs to clean", got)
	}
}

// A shared job whose main checkout the client could not resolve has nowhere to
// run, and says so. Falling back to the asking worktree would run one instance
// per worktree — the very thing sharing exists to stop.
func TestStartSharedWithoutContextIsRefused(t *testing.T) {
	f := newSharedFixture(t, nil)
	err := f.manager.Start(StartParams{Job: f.job, WorkDir: f.first})
	if !errors.Is(err, domain.ErrNoMainCheckout) {
		t.Errorf("err = %v, want ErrNoMainCheckout", err)
	}
	if got := f.processes(); got != 0 {
		t.Errorf("processes = %d, want 0: nothing may be spawned", got)
	}
}

// `run up --all` fans out over worktrees, so two of them reach the shared
// service at the same instant. One spawns it, the other must find that a
// success rather than fail its whole run.
func TestStartSharedConcurrentWorktreesSpawnOnce(t *testing.T) {
	f := newSharedFixture(t, nil)

	const worktrees = 6
	dirs := make([]string, worktrees)
	for i := range dirs {
		dirs[i] = filepath.Join(t.TempDir(), "wt")
		if err := os.MkdirAll(dirs[i], 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
	}

	errs := make([]error, worktrees)
	var wg sync.WaitGroup
	for i := range dirs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs[i] = f.start(t, dirs[i], "wt")
		}()
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Errorf("worktree %d: %v", i, err)
		}
	}
	if got := f.processes(); got != 1 {
		t.Errorf("processes = %d, want 1", got)
	}
}

// A claim owns no output. Opening a pane on it from any worktree must reach the
// one stream there is — the service's, under the main checkout's key.
func TestAttachOnAClaimReachesTheRealService(t *testing.T) {
	f := newSharedFixture(t, nil)
	f.job.Cmd = "printf 'shared-output\\n'; sleep 30"

	if err := f.start(t, f.first, "feat_a"); err != nil {
		t.Fatalf("start: %v", err)
	}

	session, err := f.manager.Attach("db", f.first)
	if err != nil {
		t.Fatalf("attach through a claim: %v", err)
	}
	defer session.Release()

	deadline := time.Now().Add(2 * time.Second)
	for !strings.Contains(string(session.History), "shared-output") && time.Now().Before(deadline) {
		select {
		case chunk := <-session.Stream:
			session.History = append(session.History, chunk...)
		case <-time.After(20 * time.Millisecond):
		}
	}
	if !strings.Contains(string(session.History), "shared-output") {
		t.Errorf("history = %q, want the shared service's output", session.History)
	}
}

// A stopped job stays in the map so `run logs` can read it back, so membership
// is not the question a restart must ask. Asking it had a service answer
// "already running" for ever once stopped, and post claims onto a corpse.
func TestStartSharedRestartsAfterAStop(t *testing.T) {
	f := newSharedFixture(t, nil)

	if err := f.start(t, f.first, "feat_a"); err != nil {
		t.Fatalf("start: %v", err)
	}
	if err := f.manager.Stop("db", f.first); err != nil {
		t.Fatalf("stop: %v", err)
	}
	if status, _ := f.statusIn(f.main); status != domain.JobStatusStopped {
		t.Fatalf("service status = %q, want stopped", status)
	}

	if err := f.start(t, f.first, "feat_a"); err != nil {
		t.Fatalf("restart: %v", err)
	}
	if status, _ := f.statusIn(f.main); status != domain.JobStatusRunning {
		t.Errorf("service status = %q, want running again", status)
	}
	if status, _ := f.statusIn(f.first); status != domain.JobStatusAttached {
		t.Errorf("claim status = %q, want attached", status)
	}
}

func TestStartSharedRestartsFromTheMainCheckoutAfterAStop(t *testing.T) {
	f := newSharedFixture(t, nil)

	if err := f.start(t, f.main, "main"); err != nil {
		t.Fatalf("start: %v", err)
	}
	if err := f.manager.Stop("db", f.main); err != nil {
		t.Fatalf("stop: %v", err)
	}
	if err := f.start(t, f.main, "main"); err != nil {
		t.Errorf("restart from the main checkout: %v", err)
	}
	if status, _ := f.statusIn(f.main); status != domain.JobStatusRunning {
		t.Errorf("status = %q, want running", status)
	}
}

// The daemon is machine-wide, so two repositories may both declare "db".
// Matching a claim to its service by name alone had one release the other's.
func TestStopSharedNeverTouchesAnotherRepositorysJobOfTheSameName(t *testing.T) {
	f := newSharedFixture(t, nil)
	foreign := t.TempDir()

	if err := f.start(t, f.first, "feat_a"); err != nil {
		t.Fatalf("start shared: %v", err)
	}
	// Another repository's ordinary job, sharing only its name.
	if err := f.manager.Start(StartParams{
		Job:     domain.JobConfig{Name: "db", Kind: domain.JobKindService, Cmd: "sleep 30"},
		WorkDir: foreign,
	}); err != nil {
		t.Fatalf("start foreign: %v", err)
	}

	if err := f.manager.Stop("db", f.first); err != nil {
		t.Fatalf("stop: %v", err)
	}

	f.manager.mu.Lock()
	foreignJob := f.manager.jobs[jobKey("db", foreign)]
	f.manager.mu.Unlock()
	if foreignJob == nil || foreignJob.Status != domain.JobStatusRunning {
		t.Errorf("the other repository's job is %v; releasing a claim must not reach it", foreignJob)
	}
}

// A foreign claim must not keep a service alive either.
func TestStopSharedIgnoresAForeignRepositorysClaims(t *testing.T) {
	f := newSharedFixture(t, nil)
	foreignMain := t.TempDir()
	foreignWork := t.TempDir()

	if err := f.start(t, f.first, "feat_a"); err != nil {
		t.Fatalf("start shared: %v", err)
	}
	if err := f.manager.Start(StartParams{
		Job:     f.job,
		WorkDir: foreignWork,
		Env:     map[string]string{domain.EnvWorktree: "other", domain.EnvOrdinal: "1"},
		Shared:  &domain.SharedJobContext{WorkDir: foreignMain, Env: map[string]string{domain.EnvWorktree: "other_main"}},
	}); err != nil {
		t.Fatalf("start foreign shared: %v", err)
	}

	if err := f.manager.Stop("db", f.first); err != nil {
		t.Fatalf("stop: %v", err)
	}
	if status, _ := f.statusIn(f.main); status != domain.JobStatusStopped {
		t.Errorf("status = %q, want stopped: the only claim on THIS service was released", status)
	}
}

// A service left running with nothing referencing it is invisible to `run ps`
// in the worktree that started it.
func TestStartSharedWithdrawsItsClaimWhenTheAttachFails(t *testing.T) {
	f := newSharedFixture(t, &domain.JobTenantConfig{Name: "t_{worktree}", Attach: "exit 9"})

	err := f.start(t, f.first, "feat_a")
	if err == nil {
		t.Fatal("a failing attach was reported as a success")
	}
	if _, held := f.statusIn(f.first); held {
		t.Error("the claim survived an attach that failed")
	}
	if status, _ := f.statusIn(f.main); status == domain.JobStatusRunning {
		t.Error("the service is still running with nothing referencing it")
	}
}
