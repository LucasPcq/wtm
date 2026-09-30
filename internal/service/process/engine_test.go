package process

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
)

func detachedStack(launcher string) domain.JobConfig {
	return domain.JobConfig{
		Name:  "stack",
		Kind:  domain.JobKindService,
		Cmd:   launcher,
		Stop:  "true",
		Ports: map[string]int{"PORT": 5000},
		URL:   &domain.JobURLConfig{Port: "PORT"},
	}
}

func stackRoutes() []domain.JobRoute {
	return []domain.JobRoute{{Job: "stack", Host: "stack.b.p.localhost", Port: "PORT"}}
}

// liveRoutes keeps what a proxy would serve: the last word on each host wins.
type liveRoutes struct {
	mu    sync.Mutex
	hosts map[string]bool
}

func newLiveRoutes() *liveRoutes { return &liveRoutes{hosts: map[string]bool{}} }

func (r *liveRoutes) Add(route domain.ProxyRoute) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.hosts[route.Host] = true
}

func (r *liveRoutes) Remove(host string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.hosts, host)
}

func (r *liveRoutes) live() map[string]bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	live := map[string]bool{}
	for host := range r.hosts {
		live[host] = true
	}
	return live
}

// A detached stack is up whatever its launcher does next: a relaunch that fails
// must not make the daemon forget the stack it is still running, nor stop
// serving its name.
func TestRelaunchOfADetachedStackThatFailsKeepsTheStack(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "ok")
	if err := os.WriteFile(marker, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	routes := newLiveRoutes()
	m := NewManagerWithRoutes(routes)
	job := detachedStack("test -f " + marker)
	logDir := filepath.Join(dir, "logs")

	if err := m.Start(StartParams{Job: job, WorkDir: dir, Routes: stackRoutes(), LogDir: logDir}); err != nil {
		t.Fatalf("first up: %v", err)
	}
	_ = os.Remove(marker)
	if err := m.Start(StartParams{Job: job, WorkDir: dir, Routes: stackRoutes(), LogDir: logDir}); err == nil {
		t.Fatal("second up succeeded, want the launcher's failure")
	}

	jobs := m.List()
	if len(jobs) != 1 || jobs[0].Status != domain.JobStatusDetached {
		t.Fatalf("jobs after a failed relaunch = %+v, want the stack still detached", jobs)
	}
	if !routes.live()["stack.b.p.localhost"] {
		t.Error("the stack's name was withdrawn, want it still served")
	}
}

func TestFailedFirstLaunchWithdrawsItsRoute(t *testing.T) {
	dir := t.TempDir()
	routes := newLiveRoutes()
	m := NewManagerWithRoutes(routes)

	if err := m.Start(StartParams{Job: detachedStack("exit 3"), WorkDir: dir, Routes: stackRoutes()}); err == nil {
		t.Fatal("start succeeded, want the launcher's failure")
	}
	if len(m.List()) != 0 {
		t.Errorf("jobs = %+v, want none", m.List())
	}
	if live := routes.live(); len(live) != 0 {
		t.Errorf("routes still served = %v, want none for a launcher that failed", live)
	}
}

// A stop reaching a launcher still running has stopped the stack. The launcher
// exiting afterwards must not bring the entry back as detached.
func TestStopDuringADetachedLauncherStaysStopped(t *testing.T) {
	dir := t.TempDir()
	m := NewManager()
	job := detachedStack("sleep 0.5")

	started := make(chan error, 1)
	go func() { started <- m.Start(StartParams{Job: job, WorkDir: dir}) }()
	waitForStatus(t, m, domain.JobStatusRunning)

	if err := m.Stop(JobRef{Name: "stack", WorkDir: dir}); err != nil {
		t.Fatalf("stop: %v", err)
	}
	<-started

	for _, j := range m.List() {
		if j.Status == domain.JobStatusDetached {
			t.Fatalf("status = detached after a stop, want it to stay stopped")
		}
	}
}

func waitForStatus(t *testing.T, m *Manager, status domain.JobStatus) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		for _, j := range m.List() {
			if j.Status == status {
				return
			}
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("no job reached %s", status)
}

// The stop command belongs to the job, so it runs where the job's own command
// ran: a `docker compose down` run one directory up finds no compose file.
func TestStopCommandRunsInTheJobsCwd(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "infra")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	m := NewManager()
	job := domain.JobConfig{Name: "stack", Kind: domain.JobKindService, Cmd: "true", Cwd: "infra", Stop: "pwd > " + filepath.Join(dir, "where")}
	if err := m.Start(StartParams{Job: job, WorkDir: dir}); err != nil {
		t.Fatalf("start: %v", err)
	}
	if err := m.Stop(JobRef{Name: "stack", WorkDir: dir}); err != nil {
		t.Fatalf("stop: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "where"))
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := filepath.EvalSymlinks(strings.TrimSpace(string(data))); got != mustEval(t, sub) {
		t.Errorf("stop ran in %q, want %q", got, sub)
	}
}

func mustEval(t *testing.T, path string) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatal(err)
	}
	return resolved
}

// A stop command that fails leaves the stack up, and so leaves it reachable.
func TestFailedStopCommandKeepsTheRoute(t *testing.T) {
	dir := t.TempDir()
	routes := newLiveRoutes()
	m := NewManagerWithRoutes(routes)
	job := detachedStack("true")
	job.Stop = "exit 4"

	if err := m.Start(StartParams{Job: job, WorkDir: dir, Routes: stackRoutes()}); err != nil {
		t.Fatalf("start: %v", err)
	}
	if err := m.Stop(JobRef{Name: "stack", WorkDir: dir}); err == nil {
		t.Fatal("stop succeeded, want the stop command's failure")
	}
	if !routes.live()["stack.b.p.localhost"] {
		t.Error("route withdrawn although the stack is still up")
	}
	if jobs := m.List(); len(jobs) != 1 || jobs[0].Status != domain.JobStatusDetached {
		t.Errorf("jobs = %+v, want the stack still detached", jobs)
	}
}

// A shared service that crashed holds nobody: a worktree that had joined it
// must be able to bring it back rather than be told it is already running.
func TestSharedServiceCrashReleasesItsClaims(t *testing.T) {
	main := t.TempDir()
	linked := t.TempDir()
	m := NewManager()
	job := domain.JobConfig{Name: "db", Kind: domain.JobKindService, Cmd: "sleep 0.2; exit 1", Scope: domain.JobScopeShared}
	shared := &domain.SharedJobContext{WorkDir: main}

	if err := m.Start(StartParams{Job: job, WorkDir: linked, Shared: shared}); err != nil {
		t.Fatalf("first up: %v", err)
	}
	waitForStatus(t, m, domain.JobStatusCrashed)

	for _, j := range m.List() {
		if j.Status == domain.JobStatusJoined {
			t.Errorf("claim of %s still joined to a crashed service", j.WorkDir)
		}
	}

	job.Cmd = "sleep 30"
	if err := m.Start(StartParams{Job: job, WorkDir: linked, Shared: shared}); err != nil {
		t.Fatalf("re-up after the crash: %v", err)
	}
	t.Cleanup(func() { _ = m.StopAll() })
}

// blockingIndex holds its first save until released, so a test can order two
// persists the way two goroutines of the daemon may.
type blockingIndex struct {
	mu      sync.Mutex
	calls   int
	entered chan struct{}
	release chan struct{}
	last    []domain.JobRecord
}

func (i *blockingIndex) Save(records []domain.JobRecord) error {
	i.mu.Lock()
	i.calls++
	first := i.calls == 1
	i.mu.Unlock()
	if first {
		close(i.entered)
		<-i.release
	}
	i.mu.Lock()
	i.last = records
	i.mu.Unlock()
	return nil
}

// The index is rewritten whole from a snapshot, so the last write must be the
// last snapshot: a slow write of an older state landing second would record a
// stack as gone while it runs.
func TestPersistNeverWritesAnOlderSnapshotLast(t *testing.T) {
	index := &blockingIndex{entered: make(chan struct{}), release: make(chan struct{})}
	m := NewManagerWith(ManagerParams{Index: index})
	add := func(name string) {
		m.mu.Lock()
		m.jobs[jobKey(name, "/w")] = &ManagedJob{Name: name, WorkDir: "/w", Status: domain.JobStatusDetached}
		m.mu.Unlock()
	}

	add("a")
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); m.persist() }()
	<-index.entered
	add("b")
	go func() { defer wg.Done(); m.persist() }()
	time.Sleep(50 * time.Millisecond)
	close(index.release)
	wg.Wait()

	if len(index.last) != 2 {
		t.Errorf("index holds %d jobs, want the latest snapshot's 2", len(index.last))
	}
}

// A shutdown is over when the jobs it stops are: the daemon exiting first kills
// a foreground service mid-cleanup and leaves the index naming it.
func TestDaemonShutdownWaitsForItsForegroundJobs(t *testing.T) {
	daemon := idleDaemon(t, time.Hour, time.Second)
	marker := filepath.Join(t.TempDir(), "cleaned")
	job := domain.JobConfig{
		Name: "slow", Kind: domain.JobKindService,
		Cmd: "trap 'sleep 0.5; touch " + marker + "; exit 0' TERM; while true; do sleep 0.05; done",
	}
	if resp := daemon.answer(t, Request{Action: ActionStart, Job: &job, WorkDir: t.TempDir()}); resp.Status != StatusOK {
		t.Fatalf("start: %s", resp.Message)
	}
	time.Sleep(200 * time.Millisecond)

	if _, err := NewClient(daemon.socket).Send(Request{Action: ActionShutdown}); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
	select {
	case <-daemon.exited:
	case <-time.After(10 * time.Second):
		t.Fatal("daemon never exited")
	}
	if _, err := os.Stat(marker); err != nil {
		t.Error("the daemon returned before its job finished stopping")
	}
	if _, err := os.Stat(daemon.socket); !os.IsNotExist(err) {
		t.Error("socket still on disk after the daemon returned")
	}
	if data, _ := os.ReadFile(StatePath()); strings.Contains(string(data), "slow") {
		t.Errorf("index still names the stopped job: %s", data)
	}
}

// Every way out of the daemon may fire at once — a signal, a shutdown request,
// the idle watcher — and only the first may run the stop.
func TestDaemonStopTwiceDoesNotPanic(t *testing.T) {
	daemon := idleDaemon(t, time.Hour, time.Second)
	var wg sync.WaitGroup
	for range 3 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = NewClient(daemon.socket).Send(Request{Action: ActionShutdown})
		}()
	}
	wg.Wait()
	select {
	case <-daemon.exited:
	case <-time.After(5 * time.Second):
		t.Fatal("daemon never exited")
	}
}

// A detached job with a name to serve keeps the proxy needed: idling the daemon
// off takes the proxy with it, and the name stops answering while the stack runs.
func TestDaemonIsNotIdleWhileItServesADetachedJob(t *testing.T) {
	m := NewManagerWithRoutes(newLiveRoutes())
	d := &daemonServer{manager: m, proxyPort: 11999}
	dir := t.TempDir()
	if err := m.Start(StartParams{Job: detachedStack("true"), WorkDir: dir, Routes: stackRoutes()}); err != nil {
		t.Fatalf("start: %v", err)
	}
	if d.idle() {
		t.Error("idle with a detached job published, want the daemon kept for the proxy")
	}
	if err := m.Stop(JobRef{Name: "stack", WorkDir: dir}); err != nil {
		t.Fatal(err)
	}
	if !d.idle() {
		t.Error("not idle once the job stopped")
	}
}
