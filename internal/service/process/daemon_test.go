package process

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
)

// jobPayloadsByName decodes a daemon response the way a client does — as JSON,
// keyed by job name — so what a job omits is asserted alongside what it
// carries: `run ps --output json` reads the absence of a key, not a zero value.
func jobPayloadsByName(t *testing.T, raw []byte) map[string]map[string]any {
	t.Helper()

	var resp struct {
		Status ResponseStatus   `json:"status"`
		Jobs   []map[string]any `json:"jobs"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		t.Fatalf("decode response %q: %v", raw, err)
	}
	if resp.Status != StatusOK {
		t.Fatalf("status = %s, want ok (%q)", resp.Status, raw)
	}

	byName := make(map[string]map[string]any, len(resp.Jobs))
	for _, job := range resp.Jobs {
		name, ok := job["name"].(string)
		if !ok {
			t.Fatalf("job payload has no name: %v", job)
		}
		byName[name] = job
	}
	return byName
}

func assertStartedAt(t *testing.T, payload map[string]any, want time.Time) {
	t.Helper()

	raw, ok := payload["started_at"].(string)
	if !ok {
		t.Fatalf("started_at = %v, want the instant the daemon spawned the job", payload["started_at"])
	}
	got, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		t.Fatalf("parse started_at %q: %v", raw, err)
	}
	if !got.Equal(want) {
		t.Errorf("started_at = %v, want %v", got, want)
	}
}

// TestDaemonHandleList_ReportsWhenAJobStartedAndHowItEnded covers the only
// bridge between a ManagedJob and the JobInfo a client renders: a field the
// daemon forgets to copy there is a column `run ps` shows empty for ever.
func TestDaemonHandleList_ReportsWhenAJobStartedAndHowItEnded(t *testing.T) {
	d := &daemonServer{manager: NewManager()}
	dir := t.TempDir()

	script := filepath.Join(dir, "boom.sh")
	if err := os.WriteFile(script, []byte("exit 7\n"), 0o755); err != nil {
		t.Fatalf("write script: %v", err)
	}

	for _, job := range []domain.JobConfig{
		{Name: "dev", Kind: domain.JobKindService, Cmd: "sleep 30"},
		{Name: "boom", Kind: domain.JobKindService, Cmd: "sh " + script},
	} {
		if err := d.manager.Start(StartParams{Job: job, WorkDir: dir}); err != nil {
			t.Fatalf("start %s: %v", job.Name, err)
		}
	}
	t.Cleanup(func() { _ = d.manager.StopAll() })

	crashed := waitForJob(t, d.manager, "boom", func(j ManagedJob) bool {
		return j.Status == domain.JobStatusCrashed
	})
	running := waitForJob(t, d.manager, "dev", func(ManagedJob) bool { return true })

	var buf bytes.Buffer
	d.handleList(replyEncoder{enc: json.NewEncoder(&buf)}, Request{Action: ActionList})
	payloads := jobPayloadsByName(t, buf.Bytes())

	dev, ok := payloads["dev"]
	if !ok {
		t.Fatalf("dev missing from %v", payloads)
	}
	assertStartedAt(t, dev, running.StartedAt)
	if code, present := dev["exit_code"]; present {
		t.Errorf("exit_code = %v, want no key at all while the job runs", code)
	}

	boom, ok := payloads["boom"]
	if !ok {
		t.Fatalf("boom missing from %v", payloads)
	}
	assertStartedAt(t, boom, crashed.StartedAt)
	if code, _ := boom["exit_code"].(float64); code != 7 {
		t.Errorf("exit_code = %v, want 7", boom["exit_code"])
	}
	if status, _ := boom["status"].(string); status != string(domain.JobStatusCrashed) {
		t.Errorf("status = %v, want crashed", boom["status"])
	}
}

// TestDaemonHandleStopAll_SnapshotsTheJobsItStopped pins that the list a
// `run down` prints back is taken before the jobs are signalled: it describes
// what was running, uptime included.
func TestDaemonHandleStopAll_SnapshotsTheJobsItStopped(t *testing.T) {
	d := &daemonServer{manager: NewManager()}
	dir := t.TempDir()

	job := domain.JobConfig{Name: "dev", Kind: domain.JobKindService, Cmd: "sleep 30"}
	if err := d.manager.Start(StartParams{Job: job, WorkDir: dir}); err != nil {
		t.Fatalf("start dev: %v", err)
	}
	t.Cleanup(func() { _ = d.manager.StopAll() })

	started := waitForJob(t, d.manager, "dev", func(ManagedJob) bool { return true })

	var buf bytes.Buffer
	d.handleStopAll(replyEncoder{enc: json.NewEncoder(&buf)}, Request{Action: ActionStopAll, WorkDir: dir})
	payloads := jobPayloadsByName(t, buf.Bytes())

	dev, ok := payloads["dev"]
	if !ok {
		t.Fatalf("dev missing from %v", payloads)
	}
	assertStartedAt(t, dev, started.StartedAt)
	if status, _ := dev["status"].(string); status != string(domain.JobStatusRunning) {
		t.Errorf("status = %v, want the running job it stopped", dev["status"])
	}
	if code, present := dev["exit_code"]; present {
		t.Errorf("exit_code = %v, want no key at all in a pre-stop snapshot", code)
	}
}

// A detached launcher's PID exited long before the row is drawn, so it must stay
// out of every state that entry can reach — not just `detached`.
func TestJobInfoReportsNoPIDForALauncherWhateverBecameOfIt(t *testing.T) {
	server := &daemonServer{manager: NewManager()}
	launcher := domain.JobConfig{Name: "db", Kind: domain.JobKindService, Cmd: "docker compose up -d", Stop: "docker compose down"}

	for _, status := range []domain.JobStatus{domain.JobStatusDetached, domain.JobStatusStopped} {
		info := server.jobInfoOf(ManagedJob{Name: "db", Config: launcher, Status: status, PID: 4058})
		if info.PID != 0 {
			t.Errorf("status %q reports pid %d: the launcher exited, so that number points at nothing or at a stranger", status, info.PID)
		}
	}
}

func TestJobInfoKeepsThePIDOfAReapedForegroundService(t *testing.T) {
	server := &daemonServer{manager: NewManager()}
	service := domain.JobConfig{Name: "dev", Kind: domain.JobKindService, Cmd: "pnpm dev"}

	info := server.jobInfoOf(ManagedJob{Name: "dev", Config: service, Status: domain.JobStatusReaped, PID: 70382})
	if info.PID != 70382 {
		t.Fatalf("pid = %d, want the group that was just reaped: it is the most useful thing on the row", info.PID)
	}
}
