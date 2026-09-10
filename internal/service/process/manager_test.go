package process

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
)

// TestManagerStartTask_StreamsOutput verifies that a one-shot task streams its
// output to the provided streamer, returns no error on a clean exit, and is
// removed from the manager afterwards.
func TestManagerStartTask_StreamsOutput(t *testing.T) {
	m := NewManager()
	dir := t.TempDir()

	var buf bytes.Buffer
	job := domain.JobConfig{Name: "greet", Kind: domain.JobKindTask, Cmd: "echo hello"}

	if err := m.Start(StartParams{Job: job, WorkDir: dir, Streamer: &buf}); err != nil {
		t.Fatalf("start task: %v", err)
	}

	if !strings.Contains(buf.String(), "hello") {
		t.Errorf("expected streamed output to contain %q, got %q", "hello", buf.String())
	}
	if len(m.List()) != 0 {
		t.Errorf("expected task to be removed after exit, still have %d job(s)", len(m.List()))
	}
}

// TestManagerStartService_AlreadyRunning verifies the daemon contract the CLI
// relies on: starting a service that is already running returns an error whose
// message ends with domain.JobAlreadyRunningSuffix, so `run up` can treat a
// repeat start as a benign no-op instead of aborting the profile.
func TestManagerStartService_AlreadyRunning(t *testing.T) {
	m := NewManager()
	dir := t.TempDir()

	job := domain.JobConfig{Name: "server", Kind: domain.JobKindService, Cmd: "sleep 30"}
	if err := m.Start(StartParams{Job: job, WorkDir: dir}); err != nil {
		t.Fatalf("first start: %v", err)
	}
	t.Cleanup(func() { _ = m.StopAll() })

	err := m.Start(StartParams{Job: job, WorkDir: dir})
	if err == nil {
		t.Fatal("expected error starting an already-running service")
	}
	if !strings.Contains(err.Error(), domain.JobAlreadyRunningSuffix) {
		t.Errorf("expected error to contain %q, got %v", domain.JobAlreadyRunningSuffix, err)
	}
}

// TestManagerStartDetached_StreamsOutput verifies that a detached launcher
// (a service with a Stop command, e.g. docker compose up -d) mirrors its
// startup output to the provided streamer live, and stays registered as
// running after the launcher process exits.
func TestManagerStartDetached_StreamsOutput(t *testing.T) {
	m := NewManager()
	dir := t.TempDir()

	var buf bytes.Buffer
	job := domain.JobConfig{Name: "compose", Kind: domain.JobKindService, Cmd: "echo creating-container", Stop: "echo down"}

	if err := m.Start(StartParams{Job: job, WorkDir: dir, Streamer: &buf}); err != nil {
		t.Fatalf("start detached: %v", err)
	}
	t.Cleanup(func() { _ = m.StopAll() })

	if !strings.Contains(buf.String(), "creating-container") {
		t.Errorf("expected streamed launcher output to contain %q, got %q", "creating-container", buf.String())
	}
	jobs := m.List()
	if len(jobs) != 1 || jobs[0].Status != domain.JobStatusDetached {
		t.Errorf("expected the launcher's exit to leave the job registered as detached, got %+v", jobs)
	}
}

// TestManagerStartDetached_FailureConciseWhenStreamed verifies that a failing
// detached launcher streams its output live and returns a CONCISE error (the
// capture is not re-embedded, since the client already saw it), and is removed.
func TestManagerStartDetached_FailureConciseWhenStreamed(t *testing.T) {
	m := NewManager()
	dir := t.TempDir()

	script := filepath.Join(dir, "up.sh")
	if err := os.WriteFile(script, []byte("echo pull-failed\nexit 1\n"), 0o755); err != nil {
		t.Fatalf("write script: %v", err)
	}

	var buf bytes.Buffer
	job := domain.JobConfig{Name: "compose", Kind: domain.JobKindService, Cmd: "sh " + script, Stop: "echo down"}

	err := m.Start(StartParams{Job: job, WorkDir: dir, Streamer: &buf})
	if err == nil {
		t.Fatal("expected error for failing detached launcher")
	}
	if !strings.Contains(buf.String(), "pull-failed") {
		t.Errorf("expected streamed output to contain %q, got %q", "pull-failed", buf.String())
	}
	if strings.Contains(err.Error(), "pull-failed") {
		t.Errorf("expected concise error without re-embedded output, got %v", err)
	}
	if len(m.List()) != 0 {
		t.Errorf("expected failed detached job to be removed, still have %d", len(m.List()))
	}
}

// TestManagerStartDetached_FailureEmbedsOutputWhenNotStreamed verifies that
// without a streamer (e.g. JSON mode) the captured output is embedded in the
// error so the failure reason still reaches the caller.
func TestManagerStartDetached_FailureEmbedsOutputWhenNotStreamed(t *testing.T) {
	m := NewManager()
	dir := t.TempDir()

	script := filepath.Join(dir, "up.sh")
	if err := os.WriteFile(script, []byte("echo pull-failed\nexit 1\n"), 0o755); err != nil {
		t.Fatalf("write script: %v", err)
	}

	job := domain.JobConfig{Name: "compose", Kind: domain.JobKindService, Cmd: "sh " + script, Stop: "echo down"}

	err := m.Start(StartParams{Job: job, WorkDir: dir})
	if err == nil {
		t.Fatal("expected error for failing detached launcher")
	}
	if !strings.Contains(err.Error(), "pull-failed") {
		t.Errorf("expected error to embed captured output, got %v", err)
	}
}

// TestManagerStartTask_FailureExitCode verifies that a failing task streams its
// output AND returns a concise error carrying the real exit code (the captured
// block is omitted because the streamer already saw it live).
func TestManagerStartTask_FailureExitCode(t *testing.T) {
	m := NewManager()
	dir := t.TempDir()

	script := filepath.Join(dir, "boom.sh")
	if err := os.WriteFile(script, []byte("echo boom\nexit 3\n"), 0o755); err != nil {
		t.Fatalf("write script: %v", err)
	}

	var buf bytes.Buffer
	job := domain.JobConfig{Name: "failer", Kind: domain.JobKindTask, Cmd: "sh " + script}

	err := m.Start(StartParams{Job: job, WorkDir: dir, Streamer: &buf})
	if err == nil {
		t.Fatal("expected error for failing task")
	}
	if !strings.Contains(buf.String(), "boom") {
		t.Errorf("expected streamed output to contain %q, got %q", "boom", buf.String())
	}
	if !strings.Contains(err.Error(), "exit 3") {
		t.Errorf("expected error to carry exit code 3, got %v", err)
	}
	// The captured output must not be re-embedded when it was streamed live.
	if strings.Contains(err.Error(), "boom") {
		t.Errorf("expected concise error without re-embedded output, got %v", err)
	}
}

func logDirFor(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "logs", "feat")
}

func waitForLogLine(t *testing.T, path string, want string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		content, err := os.ReadFile(path)
		if err == nil && strings.Contains(string(content), want) {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("log %s never contained %q (last read: %q, %v)", path, want, content, err)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// TestManagerStartTask_PersistsLog verifies that a task's output lands in its
// log file, sanitized and timestamped, on top of being streamed.
func TestManagerStartTask_PersistsLog(t *testing.T) {
	m := NewManager()
	dir := t.TempDir()
	logDir := logDirFor(t)

	var buf bytes.Buffer
	job := domain.JobConfig{Name: "greet", Kind: domain.JobKindTask, Cmd: "echo hello"}

	if err := m.Start(StartParams{Job: job, WorkDir: dir, LogDir: logDir, Streamer: &buf}); err != nil {
		t.Fatalf("start task: %v", err)
	}

	waitForLogLine(t, filepath.Join(logDir, "greet.log"), "hello")
}

// A start the manager refuses must leave the running job's log exactly as it
// was: opening a log clears it, and `run up` on a partly-started profile asks
// the daemon to start jobs that are already up (LUC-198).
func TestManagerStartRefused_LeavesTheRunningJobsLogIntact(t *testing.T) {
	m := NewManager()
	dir := t.TempDir()
	logDir := logDirFor(t)
	path := filepath.Join(logDir, "server.log")

	job := domain.JobConfig{Name: "server", Kind: domain.JobKindService, Cmd: "echo listening; sleep 30"}
	if err := m.Start(StartParams{Job: job, WorkDir: dir, LogDir: logDir}); err != nil {
		t.Fatalf("first start: %v", err)
	}
	t.Cleanup(func() { _ = m.StopAll() })
	waitForLogLine(t, path, "listening")

	if err := m.Start(StartParams{Job: job, WorkDir: dir, LogDir: logDir}); err == nil {
		t.Fatal("the second start was not refused")
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	if !strings.Contains(string(content), "listening") {
		t.Errorf("the refused start cleared the running job's log:\n%q", content)
	}
	if bytes.ContainsRune(content, 0) {
		t.Errorf("the refused start left a hole in the running job's log:\n%q", content)
	}
}

// TestManagerStartService_PersistsLog verifies that a foreground service — the
// only kind whose output is drained in the background — persists it too.
func TestManagerStartService_PersistsLog(t *testing.T) {
	m := NewManager()
	dir := t.TempDir()
	logDir := logDirFor(t)

	job := domain.JobConfig{Name: "server", Kind: domain.JobKindService, Cmd: "echo listening"}
	if err := m.Start(StartParams{Job: job, WorkDir: dir, LogDir: logDir}); err != nil {
		t.Fatalf("start service: %v", err)
	}
	t.Cleanup(func() { _ = m.StopAll() })

	waitForLogLine(t, filepath.Join(logDir, "server.log"), "listening")
}

// TestManagerStartDetached_PersistsLauncherLog verifies that a detached
// launcher, which has no output hub at all, still gets its startup output on
// disk — that log is all the user will ever have of it.
func TestManagerStartDetached_PersistsLauncherLog(t *testing.T) {
	m := NewManager()
	dir := t.TempDir()
	logDir := logDirFor(t)

	job := domain.JobConfig{Name: "compose", Kind: domain.JobKindService, Cmd: "echo creating-container", Stop: "echo down"}
	if err := m.Start(StartParams{Job: job, WorkDir: dir, LogDir: logDir}); err != nil {
		t.Fatalf("start detached: %v", err)
	}
	t.Cleanup(func() { _ = m.StopAll() })

	waitForLogLine(t, filepath.Join(logDir, "compose.log"), "creating-container")
}

// TestManagerStartWithoutLogDir_PersistsNothing pins the opt-in: a client that
// resolved no log dir gets the previous behaviour, no file written.
func TestManagerStartWithoutLogDir_PersistsNothing(t *testing.T) {
	m := NewManager()
	dir := t.TempDir()

	job := domain.JobConfig{Name: "greet", Kind: domain.JobKindTask, Cmd: "echo hello"}
	if err := m.Start(StartParams{Job: job, WorkDir: dir}); err != nil {
		t.Fatalf("start task: %v", err)
	}

	logs, err := filepath.Glob(filepath.Join(dir, "*"+domain.JobLogFileExt))
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(logs) != 0 {
		t.Errorf("found %v, want no log file", logs)
	}
}

func waitForJob(t *testing.T, m *Manager, name string, until func(ManagedJob) bool) ManagedJob {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		for _, job := range m.List() {
			if job.Name == name && until(job) {
				return job
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("job %s never reached the expected state (jobs: %+v)", name, m.List())
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// TestManagerStartService_RecordsStartedAt pins what the uptime column is read
// from: the instant the daemon spawned the process, not the instant a client
// asked for the list.
func TestManagerStartService_RecordsStartedAt(t *testing.T) {
	m := NewManager()
	dir := t.TempDir()

	before := time.Now()
	job := domain.JobConfig{Name: "server", Kind: domain.JobKindService, Cmd: "sleep 30"}
	if err := m.Start(StartParams{Job: job, WorkDir: dir}); err != nil {
		t.Fatalf("start service: %v", err)
	}
	t.Cleanup(func() { _ = m.StopAll() })

	started := m.List()[0]
	if started.StartedAt.Before(before) || started.StartedAt.After(time.Now()) {
		t.Errorf("StartedAt = %v, want between %v and now", started.StartedAt, before)
	}
	if started.ExitCode != nil {
		t.Errorf("ExitCode = %d, want nil while the job runs", *started.ExitCode)
	}
}

// TestManagerService_ReportsExitCodeOnCrash verifies that a service dying on
// its own carries the code it died with, which is what tells a crash apart
// from a clean shutdown once the process is gone.
func TestManagerService_ReportsExitCodeOnCrash(t *testing.T) {
	m := NewManager()
	dir := t.TempDir()

	script := filepath.Join(dir, "boom.sh")
	if err := os.WriteFile(script, []byte("exit 7\n"), 0o755); err != nil {
		t.Fatalf("write script: %v", err)
	}

	job := domain.JobConfig{Name: "server", Kind: domain.JobKindService, Cmd: "sh " + script}
	if err := m.Start(StartParams{Job: job, WorkDir: dir}); err != nil {
		t.Fatalf("start service: %v", err)
	}
	t.Cleanup(func() { _ = m.StopAll() })

	crashed := waitForJob(t, m, "server", func(j ManagedJob) bool {
		return j.Status == domain.JobStatusCrashed
	})
	if crashed.ExitCode == nil || *crashed.ExitCode != 7 {
		t.Errorf("ExitCode = %v, want 7", crashed.ExitCode)
	}
}

// TestManagerService_ReportsSignalExitCodeOnStop pins the -1 the JobInfo doc
// promises: a stopped job was killed, it did not choose an exit code.
func TestManagerService_ReportsSignalExitCodeOnStop(t *testing.T) {
	m := NewManager()
	dir := t.TempDir()

	job := domain.JobConfig{Name: "server", Kind: domain.JobKindService, Cmd: "sleep 30"}
	if err := m.Start(StartParams{Job: job, WorkDir: dir}); err != nil {
		t.Fatalf("start service: %v", err)
	}
	if err := m.Stop("server", dir); err != nil {
		t.Fatalf("stop service: %v", err)
	}

	stopped := m.List()[0]
	if stopped.Status != domain.JobStatusStopped {
		t.Fatalf("Status = %s, want stopped", stopped.Status)
	}
	if stopped.ExitCode == nil || *stopped.ExitCode != -1 {
		t.Errorf("ExitCode = %v, want -1 (killed by a signal)", stopped.ExitCode)
	}
}

// TestManagerStartDetached_KeepsNoExitCode covers the launcher pattern: the
// launcher exiting cleanly says nothing about the service it left running, so
// the job reports no exit code at all rather than a 0 that would read as done.
func TestManagerStartDetached_KeepsNoExitCode(t *testing.T) {
	m := NewManager()
	dir := t.TempDir()

	job := domain.JobConfig{Name: "compose", Kind: domain.JobKindService, Cmd: "echo up", Stop: "echo down"}
	if err := m.Start(StartParams{Job: job, WorkDir: dir}); err != nil {
		t.Fatalf("start detached: %v", err)
	}
	t.Cleanup(func() { _ = m.StopAll() })

	launcher := m.List()[0]
	if launcher.Status != domain.JobStatusDetached {
		t.Fatalf("Status = %s, want detached", launcher.Status)
	}
	if launcher.ExitCode != nil {
		t.Errorf("ExitCode = %d, want nil for a detached launcher", *launcher.ExitCode)
	}
	if launcher.StartedAt.IsZero() {
		t.Error("StartedAt is zero, want the launcher's spawn instant")
	}
}

// The exit code has to survive the sentence it is quoted in: the messages are
// read by people and carry no %w, so before `failure` carried it separately
// everything downstream read the 1 exitCodeOf invents for an unattributed
// failure — the JSON document's exit_code among it.
func TestExitCodeOfReadsTheCodeAFailureCarries(t *testing.T) {
	err := failure{message: "task migrate failed (exit 3)", code: 3}

	if got := exitCodeOf(err); got != 3 {
		t.Errorf("exitCodeOf = %d, want the code the failure carries", got)
	}
	if got := exitCodeOf(fmt.Errorf("wrapped: %w", err)); got != 3 {
		t.Errorf("exitCodeOf through a wrap = %d, want 3", got)
	}
	if got := exitCodeOf(errors.New("something else")); got != 1 {
		t.Errorf("exitCodeOf of an unattributed failure = %d, want 1", got)
	}
	if got := exitCodeOf(nil); got != 0 {
		t.Errorf("exitCodeOf(nil) = %d, want 0", got)
	}
}

// The window is what an error report and a pane's replay are built from, so a
// wrap must not scramble the order or lose the newest bytes — the two things a
// reader opened them for.
func TestTheOutputWindowKeepsTheLastBytesInOrder(t *testing.T) {
	cases := []struct {
		name     string
		capacity int
		chunks   []string
		want     string
	}{
		{name: "short of the window", capacity: 8, chunks: []string{"ab", "cd"}, want: "abcd"},
		{name: "exactly the window", capacity: 4, chunks: []string{"ab", "cd"}, want: "abcd"},
		{name: "wrapped once", capacity: 4, chunks: []string{"abc", "de"}, want: "bcde"},
		{name: "wrapped many times", capacity: 4, chunks: []string{"ab", "cd", "ef", "gh", "ij"}, want: "ghij"},
		{name: "a chunk bigger than the window", capacity: 4, chunks: []string{"x", "abcdefgh"}, want: "efgh"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ring := newRingBuffer(c.capacity)
			for _, chunk := range c.chunks {
				if n, err := ring.Write([]byte(chunk)); n != len(chunk) || err != nil {
					t.Fatalf("Write(%q) = %d, %v, want %d bytes taken", chunk, n, err, len(chunk))
				}
			}
			if got := ring.String(); got != c.want {
				t.Errorf("window = %q, want %q", got, c.want)
			}
			if got := string(ring.Snapshot()); got != c.want {
				t.Errorf("snapshot = %q, want %q", got, c.want)
			}
		})
	}
}

// The window is a fixed array, so a job printing megabytes must not grow it or
// recopy it: that reallocation is what made an idle daemon collect garbage.
func TestTheOutputWindowNeverGrows(t *testing.T) {
	ring := newRingBuffer(64)
	held := &ring.buf[0]

	for i := 0; i < 10_000; i++ {
		ring.Write([]byte("a line of output from a job\n"))
	}

	if len(ring.buf) != 64 || &ring.buf[0] != held {
		t.Errorf("the window moved or grew to %d bytes; it must stay the array it was allocated as", len(ring.buf))
	}
}

// The daemon stops its services on the way out and exits as soon as they report
// stopped. Reaping the process is not the end of its output — the tail is still
// being copied out of the PTY, and the sink batches its writes — so a Stop that
// returns before the drain is done lets the daemon exit on top of the shutdown
// lines the log is opened for.
func TestStoppingAServiceWaitsForItsOutputToReachTheLog(t *testing.T) {
	m := NewManager()
	dir := t.TempDir()
	logDir := filepath.Join(dir, "logs")

	job := domain.JobConfig{
		Name: "web",
		Cmd:  "trap 'echo shutting-down; exit 0' TERM; echo listening; while true; do sleep 0.05; done",
	}
	if err := m.Start(StartParams{Job: job, WorkDir: dir, LogDir: logDir}); err != nil {
		t.Fatalf("start service: %v", err)
	}
	path := JobLogPath(JobLogPathParams{LogDir: logDir, Job: "web"})
	waitForLogLine(t, path, "listening")

	m.mu.Lock()
	managed := m.jobs[jobKey("web", dir)]
	m.mu.Unlock()
	if managed == nil || managed.drained == nil {
		t.Fatal("a foreground service must carry the drain its Stop waits on")
	}

	if err := m.Stop("web", dir); err != nil {
		t.Fatalf("stop service: %v", err)
	}

	// Deterministic in the presence of the wait, and the only assertion that is:
	// racing the drain from the test would pass either way.
	select {
	case <-managed.drained:
	default:
		t.Error("Stop reported the job stopped while its last bytes were still in flight")
	}
	if logged := readLog(t, path); !strings.Contains(logged, "shutting-down") {
		t.Errorf("log = %q, want the line the job printed on its way out", logged)
	}
}
