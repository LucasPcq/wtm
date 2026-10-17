package process

import (
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/testutil/globaldir"
	"github.com/LucasPcq/wtm/internal/testutil/socktest"
)

type fakeDaemon struct {
	mu       sync.Mutex
	jobs     []domain.JobInfo
	version  string
	stopErr  string
	listErr  string
	survive  bool
	requests []Request
}

func (d *fakeDaemon) answer(req Request) Response {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.requests = append(d.requests, req)
	switch req.Action {
	case ActionList:
		if d.listErr != "" {
			return Response{Status: StatusError, Version: d.version, Message: d.listErr}
		}
		return Response{Status: StatusOK, Version: d.version, Jobs: append([]domain.JobInfo{}, d.jobs...)}
	case ActionStop, ActionStopAll:
		if d.stopErr != "" {
			return Response{Status: StatusError, Version: d.version, Message: d.stopErr}
		}
		if !d.survive {
			d.stopMatching(req)
		}
		return Response{Status: StatusOK, Version: d.version}
	}
	return Response{Status: StatusError, Version: d.version, Message: "unexpected"}
}

func (d *fakeDaemon) stopMatching(req Request) {
	for i, job := range d.jobs {
		if job.WorkDir != req.WorkDir || (req.Action == ActionStop && job.Name != req.Name) {
			continue
		}
		d.jobs[i].Status = domain.JobStatusStopped
	}
}

func (d *fakeDaemon) actions() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	var actions []string
	for _, req := range d.requests {
		if req.Action == ActionList {
			continue
		}
		actions = append(actions, string(req.Action)+":"+req.Name)
	}
	return actions
}

func serveFake(t *testing.T, daemon *fakeDaemon) string {
	t.Helper()
	globaldir.Isolate(t)
	socket := socktest.Path(t)
	socktest.Serve(t, socket, func(raw json.RawMessage) any {
		var req Request
		if err := json.Unmarshal(raw, &req); err != nil {
			return Response{Status: StatusError, Message: err.Error()}
		}
		return daemon.answer(req)
	})
	return socket
}

func worktreeJobs() []domain.JobInfo {
	return []domain.JobInfo{
		{Name: "api", Status: domain.JobStatusRunning, WorkDir: "/w/feat"},
		{Name: "stack", Status: domain.JobStatusDetached, WorkDir: "/w/feat"},
		{Name: "db", Status: domain.JobStatusJoined, WorkDir: "/w/feat"},
		{Name: "api", Status: domain.JobStatusRunning, WorkDir: "/w/other"},
	}
}

// The claim on a shared service outlives the stop: releasing the last one
// stops the service, which then could not take the worktree's data back.
func TestStopWorktreeJobsStopsItsOwnJobsAndKeepsItsClaims(t *testing.T) {
	daemon := &fakeDaemon{version: domain.Version, jobs: worktreeJobs()}
	socket := serveFake(t, daemon)

	stopped, err := StopWorktreeJobs(t.Context(), WorktreeJobsParams{SocketPath: socket, WorkDir: "/w/feat"})
	if err != nil {
		t.Fatalf("stop: %v", err)
	}
	if strings.Join(stopped, ",") != "api,stack" {
		t.Errorf("stopped = %v, want api and stack", stopped)
	}
	if got := strings.Join(daemon.actions(), " "); got != "stop:api stop:stack" {
		t.Errorf("requests = %q, want each own job stopped by name and the claim left alone", got)
	}
}

func TestStopWorktreeJobsWithNothingUpSendsNoStop(t *testing.T) {
	daemon := &fakeDaemon{version: domain.Version, jobs: []domain.JobInfo{
		{Name: "api", Status: domain.JobStatusStopped, WorkDir: "/w/feat"},
		{Name: "db", Status: domain.JobStatusJoined, WorkDir: "/w/feat"},
	}}
	socket := serveFake(t, daemon)

	stopped, err := StopWorktreeJobs(t.Context(), WorktreeJobsParams{SocketPath: socket, WorkDir: "/w/feat"})
	if err != nil || len(stopped) != 0 {
		t.Errorf("stopped = %v, %v — want nothing to do", stopped, err)
	}
	if len(daemon.actions()) != 0 {
		t.Errorf("requests = %v, want none", daemon.actions())
	}
}

// A daemon that could not be asked used to read as "no jobs", and a clean then
// removed a worktree whose API was still running.
func TestStopWorktreeJobsReportsAListingItCouldNotGet(t *testing.T) {
	daemon := &fakeDaemon{version: domain.Version, jobs: worktreeJobs(), listErr: "unknown action: list"}
	socket := serveFake(t, daemon)

	_, err := StopWorktreeJobs(t.Context(), WorktreeJobsParams{SocketPath: socket, WorkDir: "/w/feat"})
	if err == nil || !strings.Contains(err.Error(), "unknown action: list") {
		t.Fatalf("err = %v, want the daemon's refusal", err)
	}
	if len(daemon.actions()) != 0 {
		t.Errorf("requests = %v, want nothing stopped blind", daemon.actions())
	}
}

func TestStopWorktreeJobsReportsAStopTheDaemonRefused(t *testing.T) {
	daemon := &fakeDaemon{version: domain.Version, jobs: worktreeJobs(), stopErr: "compose down: exit status 1"}
	socket := serveFake(t, daemon)

	_, err := StopWorktreeJobs(t.Context(), WorktreeJobsParams{SocketPath: socket, WorkDir: "/w/feat"})
	if err == nil || !strings.Contains(err.Error(), "compose down: exit status 1") {
		t.Errorf("err = %v, want the daemon's answer", err)
	}
}

// The daemon answering "stopped" is not the job being gone.
func TestStopWorktreeJobsNamesTheJobsStillUpAfterTheStop(t *testing.T) {
	daemon := &fakeDaemon{version: domain.Version, jobs: worktreeJobs(), survive: true}
	socket := serveFake(t, daemon)

	_, err := StopWorktreeJobs(t.Context(), WorktreeJobsParams{SocketPath: socket, WorkDir: "/w/feat"})
	if !errors.Is(err, domain.ErrWorktreeJobsRunning) || !strings.Contains(err.Error(), "api, stack") {
		t.Errorf("err = %v, want the survivors named", err)
	}
}

func TestStopWorktreeJobsWithoutADaemonOrAnIndexHasNothingToDo(t *testing.T) {
	globaldir.Isolate(t)

	stopped, err := StopWorktreeJobs(t.Context(), WorktreeJobsParams{SocketPath: socktest.Path(t), WorkDir: "/w/feat"})
	if err != nil || len(stopped) != 0 {
		t.Errorf("stopped = %v, %v — want nothing, and no daemon started for it", stopped, err)
	}
}

func TestReleaseWorktreeJobsLetsGoOfEverythingLeft(t *testing.T) {
	daemon := &fakeDaemon{version: domain.Version, jobs: worktreeJobs()}
	socket := serveFake(t, daemon)

	if err := ReleaseWorktreeJobs(t.Context(), WorktreeJobsParams{SocketPath: socket, WorkDir: "/w/feat"}); err != nil {
		t.Fatalf("release: %v", err)
	}
	if got := strings.Join(daemon.actions(), " "); got != "stop_all:" {
		t.Errorf("requests = %q, want one stop of the whole worktree", got)
	}
}

func TestReleaseWorktreeJobsReportsARefusal(t *testing.T) {
	daemon := &fakeDaemon{version: domain.Version, jobs: worktreeJobs(), stopErr: "boom"}
	socket := serveFake(t, daemon)

	if err := ReleaseWorktreeJobs(t.Context(), WorktreeJobsParams{SocketPath: socket, WorkDir: "/w/feat"}); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Errorf("err = %v, want the refusal", err)
	}
}
