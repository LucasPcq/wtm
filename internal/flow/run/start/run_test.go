package start_test

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/flow/run/seam"
	"github.com/LucasPcq/wtm/internal/flow/run/start"
	"github.com/LucasPcq/wtm/internal/flow/runlogs"
	"github.com/LucasPcq/wtm/internal/service/process/processtest"
	"github.com/LucasPcq/wtm/internal/testutil/flowtest"
	"github.com/LucasPcq/wtm/internal/testutil/gittest"
)

var declared = domain.RunConfig{Jobs: []domain.JobConfig{
	{Name: "api", Kind: domain.JobKindService, Cmd: "pnpm dev"},
	{Name: "migrate", Kind: domain.JobKindTask, Cmd: "pnpm migrate"},
}}

// watcher is the surface: it records what it was handed and, when asked to,
// drives the start sequence against the daemon.
type watcher struct {
	*flowtest.Recorder
	drive  bool
	called *seam.SequenceParams
}

func (w *watcher) Sequence(params seam.SequenceParams) (runlogs.Outcomes, error) {
	w.called = &params
	if !w.drive {
		return runlogs.Outcomes{{}}, nil
	}
	return params.Start(context.Background(), nil)
}

type fixture struct {
	repo   string
	daemon *processtest.Daemon
}

func newFixture(t *testing.T, running ...domain.JobInfo) fixture {
	t.Helper()
	daemon := processtest.Serve(t, running)
	repo, err := filepath.EvalSymlinks(gittest.InitRepo(t))
	if err != nil {
		t.Fatal(err)
	}
	return fixture{repo: repo, daemon: daemon}
}

func (f fixture) run(t *testing.T, request start.Request, presenter *watcher) (start.Outcome, error) {
	t.Helper()
	request.Cwd = f.repo
	if request.Config.Jobs == nil {
		request.Config = declared
	}
	return start.Run(start.Params{
		Context:   flow.Context{ProjectDir: f.repo, StateDir: filepath.Join(f.repo, ".git", "wtm")},
		Request:   request,
		Prompter:  flow.Unattended{},
		Presenter: presenter,
	})
}

func TestStartHandsTheJobToTheSurfaceAndTheDaemon(t *testing.T) {
	f := newFixture(t)
	presenter := &watcher{Recorder: &flowtest.Recorder{}, drive: true}

	outcome, err := f.run(t, start.Request{Job: "api", NoProbe: true}, presenter)

	if err != nil {
		t.Fatalf("start: %v", err)
	}
	if presenter.called == nil || presenter.called.Job != "api" || presenter.called.Inline {
		t.Fatalf("sequence = %+v, want api handed over as a service", presenter.called)
	}
	if outcome.WorkDir != f.repo || outcome.Job.Name != "api" {
		t.Errorf("outcome = %+v, want api in %s", outcome, f.repo)
	}
	if want := "start:@" + f.repo; strings.Join(f.daemon.Actions(), " ") != want {
		t.Errorf("requests = %v, want %s", f.daemon.Actions(), want)
	}
}

// A task runs to its end in the scrollback: the flow says so, since it is the
// one that resolved the job.
func TestStartOfATaskRunsInline(t *testing.T) {
	f := newFixture(t)
	presenter := &watcher{Recorder: &flowtest.Recorder{}}

	if _, err := f.run(t, start.Request{Job: "migrate"}, presenter); err != nil {
		t.Fatalf("start: %v", err)
	}
	if presenter.called == nil || !presenter.called.Inline {
		t.Errorf("sequence = %+v, want the task inline", presenter.called)
	}
}

// A name run.toml does not declare fails before the daemon is woken.
func TestStartRefusesAnUndeclaredJob(t *testing.T) {
	f := newFixture(t)
	presenter := &watcher{Recorder: &flowtest.Recorder{}}

	_, err := f.run(t, start.Request{Job: "nope"}, presenter)

	if !errors.Is(err, domain.ErrJobNotFound) {
		t.Fatalf("err = %v, want ErrJobNotFound", err)
	}
	if len(presenter.Stages) != 0 || presenter.called != nil {
		t.Errorf("stages = %v, sequence = %v: nothing should have been reached", presenter.Stages, presenter.called)
	}
}

// Nobody to ask which job: the run refuses naming the flag rather than picking.
func TestAnUnattendedStartWithNoJobNamesTheFlag(t *testing.T) {
	f := newFixture(t)

	_, err := f.run(t, start.Request{}, &watcher{Recorder: &flowtest.Recorder{}})

	if err == nil || !strings.Contains(err.Error(), "--"+domain.FlagJob) {
		t.Fatalf("err = %v, want --%s named", err, domain.FlagJob)
	}
}

// A runner and one of its own children are the same process twice: starting
// one while the other is up is refused before anything starts.
func TestStartRefusesAJobItsRunnerAlreadyStarted(t *testing.T) {
	cfg := domain.RunConfig{Jobs: []domain.JobConfig{
		{Name: "dev", Kind: domain.JobKindService, Cmd: "turbo dev", Runs: []string{"api"}},
		{Name: "api", Kind: domain.JobKindService, Cmd: "pnpm dev"},
	}}
	f := newFixture(t)
	f.daemon = processtest.Serve(t, []domain.JobInfo{{Name: "dev", Status: domain.JobStatusRunning, WorkDir: f.repo}})
	presenter := &watcher{Recorder: &flowtest.Recorder{}}

	_, err := f.run(t, start.Request{Job: "api", Config: cfg}, presenter)

	if err == nil || !strings.Contains(err.Error(), domain.JobConflictTitle) {
		t.Fatalf("err = %v, want the conflict refused", err)
	}
	if presenter.called != nil {
		t.Error("a refused start reached the surface")
	}
}

func TestStartBackedOutOfSaysAborted(t *testing.T) {
	f := newFixture(t)
	presenter := &watcher{Recorder: &flowtest.Recorder{}}

	outcome, err := start.Run(start.Params{
		Context:   flow.Context{ProjectDir: f.repo},
		Request:   start.Request{Cwd: f.repo, Config: declared},
		Prompter:  &flowtest.ScriptedPrompter{Abort: true},
		Presenter: presenter,
	})

	if err != nil || !outcome.Aborted {
		t.Fatalf("outcome = %+v, err = %v, want an abort", outcome, err)
	}
	if presenter.called != nil {
		t.Error("an abort reached the surface")
	}
}
