package stop_test

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/flow/run/stop"
	"github.com/LucasPcq/wtm/internal/service/process/processtest"
	"github.com/LucasPcq/wtm/internal/testutil/flowtest"
	"github.com/LucasPcq/wtm/internal/testutil/gittest"
	"github.com/LucasPcq/wtm/internal/testutil/globaldir"
)

var declared = domain.RunConfig{Jobs: []domain.JobConfig{
	{Name: "api", Kind: domain.JobKindService, Cmd: "pnpm dev"},
	{Name: "web", Kind: domain.JobKindService, Cmd: "pnpm web"},
}}

type recorder struct {
	*flowtest.Recorder
	stopped *stop.Outcome
}

func (r *recorder) Stopped(outcome stop.Outcome) error {
	r.stopped = &outcome
	return nil
}

// project is a repository whose root is spelled the way git spells it, which
// is how the daemon keys a job.
func project(t *testing.T) string {
	t.Helper()
	repo, err := filepath.EvalSymlinks(gittest.InitRepo(t))
	if err != nil {
		t.Fatal(err)
	}
	return repo
}

func run(t *testing.T, repo string, request stop.Request) (stop.Outcome, *recorder, error) {
	t.Helper()
	presenter := &recorder{Recorder: &flowtest.Recorder{}}
	request.Cwd = repo
	if request.Config.Jobs == nil && !request.ByName {
		request.Config = declared
	}
	outcome, err := stop.Run(t.Context(), stop.Params{
		Context:   flow.Context{ProjectDir: repo},
		Request:   request,
		Prompter:  flow.Unattended{},
		Presenter: presenter,
	})
	return outcome, presenter, err
}

func only(t *testing.T, outcome stop.Outcome) domain.JobActionResult {
	t.Helper()
	if len(outcome.Results) != 1 || len(outcome.Results[0].Jobs) != 1 {
		t.Fatalf("results = %+v, want one worktree holding one job", outcome.Results)
	}
	return outcome.Results[0].Jobs[0]
}

func TestStopStopsTheJobUpInTheWorktree(t *testing.T) {
	repo := project(t)
	daemon := processtest.Serve(t, []domain.JobInfo{{Name: "api", Status: domain.JobStatusRunning, WorkDir: repo}})

	outcome, presenter, err := run(t, repo, stop.Request{Job: "api"})

	if err != nil {
		t.Fatalf("stop: %v", err)
	}
	if got := only(t, outcome); got.Status != domain.JobActionStopped {
		t.Errorf("status = %q, want stopped", got.Status)
	}
	if outcome.Results[0].Branch != "main" || outcome.Results[0].Path != repo {
		t.Errorf("worktree = %s @ %s, want main @ %s", outcome.Results[0].Branch, outcome.Results[0].Path, repo)
	}
	if want := "stop:api@" + repo; strings.Join(daemon.Actions(), " ") != want {
		t.Errorf("requests = %v, want %s", daemon.Actions(), want)
	}
	if presenter.stopped == nil {
		t.Error("the outcome never reached the presenter")
	}
}

// A job that was not up was not stopped: the daemon is not asked, and the
// result says so rather than claiming a stop.
func TestStopReportsAJobThatWasNotUpAsNotRunning(t *testing.T) {
	repo := project(t)
	daemon := processtest.Serve(t, []domain.JobInfo{{Name: "web", Status: domain.JobStatusRunning, WorkDir: repo}})

	outcome, _, err := run(t, repo, stop.Request{Job: "api"})

	if err != nil {
		t.Fatalf("stop: %v", err)
	}
	if got := only(t, outcome); got.Status != domain.JobActionNotRunning {
		t.Errorf("status = %q, want not_running", got.Status)
	}
	if len(daemon.Actions()) != 0 {
		t.Errorf("requests = %v, want the daemon left alone", daemon.Actions())
	}
}

func TestStopOfASharedJobLetGoOfIsReleased(t *testing.T) {
	repo := project(t)
	daemon := processtest.Serve(t, []domain.JobInfo{{Name: "api", Status: domain.JobStatusJoined, WorkDir: repo}})
	daemon.Release = true

	outcome, _, err := run(t, repo, stop.Request{Job: "api"})

	if err != nil {
		t.Fatalf("stop: %v", err)
	}
	if got := only(t, outcome); got.Status != domain.JobActionReleased {
		t.Errorf("status = %q, want released", got.Status)
	}
}

// With no daemon and nothing indexed, nothing can be up: the command still
// answers, every job not_running, and starts no daemon to say so.
func TestStopWithNoDaemonAnswersNotRunning(t *testing.T) {
	globaldir.Isolate(t)
	repo := project(t)

	outcome, _, err := run(t, repo, stop.Request{Job: "api"})

	if err != nil {
		t.Fatalf("stop: %v", err)
	}
	if !outcome.NoDaemon {
		t.Error("NoDaemon = false, want the absence said")
	}
	if got := only(t, outcome); got.Status != domain.JobActionNotRunning || got.Name != "api" {
		t.Errorf("result = %+v, want api not_running", got)
	}
}

// A name run.toml does not declare fails before the daemon is asked anything.
func TestStopRefusesAnUndeclaredJob(t *testing.T) {
	repo := project(t)
	daemon := processtest.Serve(t, nil)

	_, _, err := run(t, repo, stop.Request{Job: "nope"})

	if !errors.Is(err, domain.ErrJobNotFound) {
		t.Fatalf("err = %v, want ErrJobNotFound", err)
	}
	if len(daemon.Actions()) != 0 {
		t.Errorf("requests = %v, want none", daemon.Actions())
	}
}

// Stopping never depends on run.toml: an unreadable one stops by the name given.
func TestStopByNameStopsWhatRunTomlCannotDeclare(t *testing.T) {
	repo := project(t)
	daemon := processtest.Serve(t, []domain.JobInfo{{Name: "ghost", Status: domain.JobStatusRunning, WorkDir: repo}})

	outcome, _, err := run(t, repo, stop.Request{Job: "ghost", ByName: true})

	if err != nil {
		t.Fatalf("stop: %v", err)
	}
	if got := only(t, outcome); got.Status != domain.JobActionStopped {
		t.Errorf("status = %q, want stopped", got.Status)
	}
	if want := "stop:ghost@" + repo; strings.Join(daemon.Actions(), " ") != want {
		t.Errorf("requests = %v, want %s", daemon.Actions(), want)
	}
}

func TestStopFailsOnAStopTheDaemonRefused(t *testing.T) {
	repo := project(t)
	daemon := processtest.Serve(t, []domain.JobInfo{{Name: "api", Status: domain.JobStatusRunning, WorkDir: repo}})
	daemon.StopError = "job api refused to stop"

	_, presenter, err := run(t, repo, stop.Request{Job: "api"})

	if err == nil || !strings.Contains(err.Error(), "refused to stop") {
		t.Fatalf("err = %v, want the daemon's refusal", err)
	}
	if presenter.stopped != nil {
		t.Error("a failed stop reached the presenter as a conclusion")
	}
}

// The same job is stopped in every worktree named, each reported under its own
// branch.
func TestStopActsOnEveryWorktreeNamed(t *testing.T) {
	repo := project(t)
	second := filepath.Join(t.TempDir(), "feature")
	gittest.Git(t, repo, "worktree", "add", "-b", "feature", second)
	second, _ = filepath.EvalSymlinks(second)
	daemon := processtest.Serve(t, []domain.JobInfo{
		{Name: "api", Status: domain.JobStatusRunning, WorkDir: repo},
		{Name: "api", Status: domain.JobStatusRunning, WorkDir: second},
	})

	outcome, _, err := run(t, repo, stop.Request{Job: "api", Worktrees: []string{"main", "feature"}})

	if err != nil {
		t.Fatalf("stop: %v", err)
	}
	if len(outcome.Results) != 2 || outcome.Results[0].Branch != "main" || outcome.Results[1].Branch != "feature" {
		t.Fatalf("results = %+v, want main then feature", outcome.Results)
	}
	if want := "stop:api@" + repo + " stop:api@" + second; strings.Join(daemon.Actions(), " ") != want {
		t.Errorf("requests = %v, want %s", daemon.Actions(), want)
	}
}

func TestStopBackedOutOfSaysAborted(t *testing.T) {
	repo := project(t)
	processtest.Serve(t, nil)
	presenter := &recorder{Recorder: &flowtest.Recorder{}}

	outcome, err := stop.Run(t.Context(), stop.Params{
		Context:   flow.Context{ProjectDir: repo},
		Request:   stop.Request{Cwd: repo, Config: declared},
		Prompter:  &flowtest.ScriptedPrompter{Abort: true},
		Presenter: presenter,
	})

	if err != nil || !outcome.Aborted {
		t.Fatalf("outcome = %+v, err = %v, want an abort", outcome, err)
	}
	if len(presenter.Notices) != 1 || presenter.Notices[0].Text != flow.AbortedNotice.Text {
		t.Errorf("notices = %+v, want Aborted", presenter.Notices)
	}
}
