package teardown_test

import (
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/flow/teardown"
	"github.com/LucasPcq/wtm/internal/service/process/processtest"
	"github.com/LucasPcq/wtm/internal/testutil/flowtest"
	"github.com/LucasPcq/wtm/internal/testutil/globaldir"
)

var target = teardown.Target{Branch: "feat/x", Path: "/trees/feat-x"}

func TestStopSaysSoOnlyWhenSomethingWasStopped(t *testing.T) {
	processtest.Serve(t, []domain.JobInfo{{Name: "api", Status: domain.JobStatusRunning, WorkDir: target.Path}})
	presenter := &flowtest.Recorder{}

	if err := teardown.Stop(teardown.StopParams{Presenter: presenter, Target: target}); err != nil {
		t.Fatalf("stop: %v", err)
	}
	if len(presenter.Statuses) != 1 || presenter.Statuses[0].Kind != flow.NoticeSuccess {
		t.Errorf("statuses = %+v, want the stop said once", presenter.Statuses)
	}

	quiet := &flowtest.Recorder{}
	if err := teardown.Stop(teardown.StopParams{Presenter: quiet, Target: target}); err != nil {
		t.Fatalf("second stop: %v", err)
	}
	if len(quiet.Statuses) != 0 {
		t.Errorf("statuses = %+v, want nothing said for nothing stopped", quiet.Statuses)
	}
}

// A worktree git no longer knows has no path, and nothing of its own to stop.
func TestStopOfAWorktreeWithNoPathAsksNothing(t *testing.T) {
	daemon := processtest.Serve(t, nil)

	if err := teardown.Stop(teardown.StopParams{Presenter: &flowtest.Recorder{}, Target: teardown.Target{Branch: "gone"}}); err != nil {
		t.Fatalf("stop: %v", err)
	}
	if len(daemon.Actions()) != 0 {
		t.Errorf("requests = %v, want none", daemon.Actions())
	}
}

func TestStopWithNoDaemonAndNothingIndexedStartsNone(t *testing.T) {
	globaldir.Isolate(t)

	if err := teardown.Stop(teardown.StopParams{Presenter: &flowtest.Recorder{}, Target: target}); err != nil {
		t.Fatalf("stop: %v", err)
	}
}

// A claim that could not be let go of is said, not fatal: the removal already
// happened.
func TestReleaseRefusedIsAWarning(t *testing.T) {
	daemon := processtest.Serve(t, []domain.JobInfo{{Name: "postgres", Status: domain.JobStatusJoined, WorkDir: target.Path}})
	daemon.StopError = "daemon busy"
	presenter := &flowtest.Recorder{}

	teardown.Release(teardown.ReleaseParams{Presenter: presenter, Target: target})

	if len(presenter.Statuses) != 1 || presenter.Statuses[0].Kind != flow.NoticeWarning || !strings.Contains(presenter.Statuses[0].Text, "daemon busy") {
		t.Errorf("statuses = %+v, want the refusal warned about", presenter.Statuses)
	}
}

func TestReleaseLetsGoOfEverythingTheWorktreeHolds(t *testing.T) {
	daemon := processtest.Serve(t, []domain.JobInfo{{Name: "postgres", Status: domain.JobStatusJoined, WorkDir: target.Path}})
	presenter := &flowtest.Recorder{}

	teardown.Release(teardown.ReleaseParams{Presenter: presenter, Target: target})

	if want := "stop_all:@" + target.Path; strings.Join(daemon.Actions(), " ") != want {
		t.Errorf("requests = %v, want %s", daemon.Actions(), want)
	}
	if len(presenter.Statuses) != 0 {
		t.Errorf("statuses = %+v, want none", presenter.Statuses)
	}
}
