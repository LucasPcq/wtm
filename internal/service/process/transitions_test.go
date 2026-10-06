package process

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
)

type transitionLog struct {
	mu   sync.Mutex
	seen []JobTransition
}

func (l *transitionLog) record(transition JobTransition) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.seen = append(l.seen, transition)
}

func (l *transitionLog) all() []JobTransition {
	l.mu.Lock()
	defer l.mu.Unlock()
	return slices.Clone(l.seen)
}

func (l *transitionLog) types() []domain.EventType {
	var types []domain.EventType
	for _, transition := range l.all() {
		types = append(types, transition.Type)
	}
	return types
}

func (l *transitionLog) await(t *testing.T, count int) []JobTransition {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if seen := l.all(); len(seen) >= count {
			return seen
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("heard %v, want %d transitions", l.types(), count)
	return nil
}

func newObservedManager() (*Manager, *transitionLog) {
	log := &transitionLog{}
	return NewManagerWith(ManagerParams{OnTransition: log.record}), log
}

var testOrigin = &domain.EventOrigin{
	Repo:          domain.EventRepo{Root: "/code/app", CommonDir: "/code/app/.git"},
	CorrelationID: "run-up-1",
}

func TestAServiceThatDiesOnItsOwnIsHeardStartedThenCrashed(t *testing.T) {
	m, log := newObservedManager()
	job := domain.JobConfig{Name: "web", Kind: domain.JobKindService, Cmd: "echo listening; sleep 0.2; echo 'Error: boom'; exit 3"}
	if err := m.Start(StartParams{Job: job, WorkDir: t.TempDir(), Origin: testOrigin}); err != nil {
		t.Fatal(err)
	}

	seen := log.await(t, 2)
	if got := log.types(); !slices.Equal(got, []domain.EventType{domain.EventJobStarted, domain.EventJobCrashed}) {
		t.Fatalf("heard %v", got)
	}
	crash := seen[1]
	if crash.ExitCode == nil || *crash.ExitCode != 3 {
		t.Fatalf("exit code = %v, want 3", crash.ExitCode)
	}
	if !slices.Contains(crash.LastLines, "Error: boom") {
		t.Fatalf("last lines = %q, want the error the job printed", crash.LastLines)
	}
	if crash.CorrelationID != "run-up-1" || crash.Job.Origin != testOrigin {
		t.Fatalf("crash carries correlation %q and origin %v, want the start's", crash.CorrelationID, crash.Job.Origin)
	}
}

// A stop kills the process, whose exit the reaper used to read as a crash
// before the stop marked it stopped.
func TestAStoppedServiceIsHeardStoppedNeverCrashed(t *testing.T) {
	m, log := newObservedManager()
	dir := t.TempDir()
	job := domain.JobConfig{Name: "web", Kind: domain.JobKindService, Cmd: "sleep 30"}
	if err := m.Start(StartParams{Job: job, WorkDir: dir, Origin: testOrigin}); err != nil {
		t.Fatal(err)
	}
	m.AttributeStop(AttributeStopParams{WorkDir: dir, Name: "web", CorrelationID: "run-stop-1"})
	if err := m.Stop(JobRef{Name: "web", WorkDir: dir}); err != nil {
		t.Fatal(err)
	}

	seen := log.await(t, 2)
	time.Sleep(100 * time.Millisecond)
	if got := log.types(); !slices.Equal(got, []domain.EventType{domain.EventJobStarted, domain.EventJobStopped}) {
		t.Fatalf("heard %v, want started then stopped", got)
	}
	if seen[1].CorrelationID != "run-stop-1" {
		t.Fatalf("stopped carries %q, want the stop's correlation id", seen[1].CorrelationID)
	}
	if seen[0].CorrelationID != "run-up-1" {
		t.Fatalf("started carries %q, want the start's correlation id", seen[0].CorrelationID)
	}
}

func TestATaskIsHeardExitedOrCrashed(t *testing.T) {
	cases := []struct {
		name string
		cmd  string
		want domain.EventType
		code int
	}{
		{"a task that succeeds", "echo done", domain.EventJobExited, 0},
		{"a task that fails", "echo 'migration failed'; exit 2", domain.EventJobCrashed, 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m, log := newObservedManager()
			job := domain.JobConfig{Name: "migrate", Kind: domain.JobKindTask, Cmd: tc.cmd}
			_ = m.Start(StartParams{Job: job, WorkDir: t.TempDir(), Origin: testOrigin})

			seen := log.await(t, 2)
			if got := log.types(); !slices.Equal(got, []domain.EventType{domain.EventJobStarted, tc.want}) {
				t.Fatalf("heard %v", got)
			}
			if seen[1].ExitCode == nil || *seen[1].ExitCode != tc.code {
				t.Fatalf("exit code = %v, want %d", seen[1].ExitCode, tc.code)
			}
			if tc.want == domain.EventJobCrashed && !slices.Contains(seen[1].LastLines, "migration failed") {
				t.Fatalf("last lines = %q", seen[1].LastLines)
			}
		})
	}
}

func TestADetachedStackIsHeardStartedOnceItsLauncherExits(t *testing.T) {
	m, log := newObservedManager()
	dir := t.TempDir()
	job := domain.JobConfig{Name: "db", Kind: domain.JobKindService, Cmd: "echo up", Stop: "echo down"}
	if err := m.Start(StartParams{Job: job, WorkDir: dir, Origin: testOrigin}); err != nil {
		t.Fatal(err)
	}
	if err := m.Stop(JobRef{Name: "db", WorkDir: dir}); err != nil {
		t.Fatal(err)
	}
	log.await(t, 2)
	if got := log.types(); !slices.Equal(got, []domain.EventType{domain.EventJobStarted, domain.EventJobStopped}) {
		t.Fatalf("heard %v", got)
	}
}

func TestAFailingLauncherIsHeardCrashed(t *testing.T) {
	m, log := newObservedManager()
	job := domain.JobConfig{Name: "db", Kind: domain.JobKindService, Cmd: "echo 'port is already allocated'; exit 1", Stop: "echo down"}
	if err := m.Start(StartParams{Job: job, WorkDir: t.TempDir(), Origin: testOrigin}); err == nil {
		t.Fatal("a failing launcher started")
	}
	seen := log.await(t, 1)
	if seen[0].Type != domain.EventJobCrashed || !strings.Contains(strings.Join(seen[0].LastLines, "\n"), "already allocated") {
		t.Fatalf("heard %+v, want a crash naming the launcher's error", seen[0])
	}
}

func TestTheOriginSurvivesTheIndex(t *testing.T) {
	m := newManager()
	dir := t.TempDir()
	job := domain.JobConfig{Name: "db", Kind: domain.JobKindService, Cmd: "echo up", Stop: "echo down"}
	if err := m.Start(StartParams{Job: job, WorkDir: dir, Origin: testOrigin}); err != nil {
		t.Fatal(err)
	}
	m.mu.Lock()
	records := m.upRecordsLocked()
	m.mu.Unlock()
	if len(records) != 1 || records[0].Origin == nil || *records[0].Origin != *testOrigin {
		t.Fatalf("indexed %+v, want the origin kept", records)
	}

	next := NewManagerWith(ManagerParams{Stacks: &unknownStacks{}, Orphans: &fakeOrphans{}})
	next.Adopt(records)
	jobs := next.List()
	if len(jobs) != 1 || jobs[0].Origin == nil || *jobs[0].Origin != *testOrigin {
		t.Fatalf("adopted %+v, want the origin back", jobs)
	}
}

type observedShared struct {
	manager *Manager
	log     *transitionLog
	main    string
	first   string
	second  string
	job     domain.JobConfig
}

func newObservedShared(t *testing.T, cmd string) observedShared {
	t.Helper()
	root := t.TempDir()
	manager, log := newObservedManager()
	shared := observedShared{
		manager: manager,
		log:     log,
		main:    filepath.Join(root, "main"),
		first:   filepath.Join(root, "feat-a"),
		second:  filepath.Join(root, "feat-b"),
		job:     domain.JobConfig{Name: "db", Kind: domain.JobKindService, Cmd: cmd, Scope: domain.JobScopeShared},
	}
	for _, dir := range []string{shared.main, shared.first, shared.second} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() { _ = manager.StopAll() })
	return shared
}

func (s observedShared) start(t *testing.T, workDir, branch string) {
	t.Helper()
	err := s.manager.Start(StartParams{
		Job:     s.job,
		WorkDir: workDir,
		Env:     map[string]string{domain.EnvBranch: branch},
		Origin:  testOrigin,
		Shared:  &domain.SharedJobContext{WorkDir: s.main, Env: map[string]string{domain.EnvBranch: "main"}},
	})
	if err != nil {
		t.Fatalf("start from %s: %v", branch, err)
	}
}

func (s observedShared) ref(branch string) domain.WorktreeRef {
	paths := map[string]string{"main": s.main, "feat/a": s.first, "feat/b": s.second}
	return domain.WorktreeRef{Branch: branch, Path: paths[branch]}
}

func TestASharedServiceStartedForAWorktreeIsHeldByIt(t *testing.T) {
	s := newObservedShared(t, "sleep 30")
	s.start(t, s.first, "feat/a")
	s.start(t, s.second, "feat/b")

	seen := s.log.await(t, 1)
	time.Sleep(100 * time.Millisecond)
	if got := s.log.types(); !slices.Equal(got, []domain.EventType{domain.EventJobStarted}) {
		t.Fatalf("heard %v, want one started for two worktrees", got)
	}
	if seen[0].Job.WorkDir != s.main || !slices.Equal(seen[0].HeldBy, []domain.WorktreeRef{s.ref("feat/a")}) {
		t.Fatalf("started in %s held by %+v, want main held by feat/a", seen[0].Job.WorkDir, seen[0].HeldBy)
	}
}

func TestASharedServiceCrashIsHeardByEveryWorktreeHoldingIt(t *testing.T) {
	s := newObservedShared(t, "sleep 1; exit 5")
	s.start(t, s.first, "feat/a")
	s.start(t, s.second, "feat/b")

	seen := s.log.await(t, 2)
	crash := seen[1]
	if crash.Type != domain.EventJobCrashed {
		t.Fatalf("heard %v", s.log.types())
	}
	if want := []domain.WorktreeRef{s.ref("feat/a"), s.ref("feat/b")}; !slices.Equal(crash.HeldBy, want) {
		t.Fatalf("crash held by %+v, want %+v", crash.HeldBy, want)
	}
}

func TestASharedServiceStopIsHeardByTheWorktreeThatLetItGo(t *testing.T) {
	s := newObservedShared(t, "sleep 30")
	s.start(t, s.first, "feat/a")
	s.start(t, s.second, "feat/b")
	if err := s.manager.Stop(JobRef{Name: "db", WorkDir: s.first}); err != nil {
		t.Fatal(err)
	}
	if err := s.manager.Stop(JobRef{Name: "db", WorkDir: s.second}); err != nil {
		t.Fatal(err)
	}

	seen := s.log.await(t, 2)
	time.Sleep(100 * time.Millisecond)
	if got := s.log.types(); !slices.Equal(got, []domain.EventType{domain.EventJobStarted, domain.EventJobStopped}) {
		t.Fatalf("heard %v, want one stop once nobody holds it", got)
	}
	if !slices.Equal(seen[1].HeldBy, []domain.WorktreeRef{s.ref("feat/b")}) {
		t.Fatalf("stop held by %+v, want feat/b, whose release stopped it", seen[1].HeldBy)
	}
}

func TestAJobThatIsNotSharedIsHeldByNobody(t *testing.T) {
	m, log := newObservedManager()
	job := domain.JobConfig{Name: "migrate", Kind: domain.JobKindTask, Cmd: "true"}
	_ = m.Start(StartParams{Job: job, WorkDir: t.TempDir(), Origin: testOrigin})
	for _, transition := range log.await(t, 2) {
		if transition.HeldBy != nil {
			t.Fatalf("%s held by %+v, want nil", transition.Type, transition.HeldBy)
		}
	}
}
